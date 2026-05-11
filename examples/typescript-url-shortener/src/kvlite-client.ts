import * as net from 'net';
import { EventEmitter } from 'events';
import { config } from './config';

// ANSI colors for console output
const colors = {
  reset: '\x1b[0m',
  cyan: '\x1b[36m',
  green: '\x1b[32m',
  yellow: '\x1b[33m',
  red: '\x1b[31m',
  gray: '\x1b[90m',
};

class KVLiteLogger {
  private enabled: boolean;

  constructor(enabled: boolean = true) {
    this.enabled = enabled;
  }

  private timestamp(): string {
    return new Date().toISOString().split('T')[1].slice(0, 12);
  }

  command(cmd: string): void {
    if (!this.enabled) return;
    console.log(
      `${colors.gray}[${this.timestamp()}]${colors.reset} ${colors.cyan}KVLITE >>>>${colors.reset} ${cmd}`
    );
  }

  response(resp: string, durationMs: number): void {
    if (!this.enabled) return;
    const color = resp.startsWith('-ERR') ? colors.red : colors.green;
    console.log(
      `${colors.gray}[${this.timestamp()}]${colors.reset} ${color}KVLITE <<<<${colors.reset} ${resp} ${colors.gray}(${durationMs.toFixed(2)}ms)${colors.reset}`
    );
  }

  info(msg: string): void {
    if (!this.enabled) return;
    console.log(
      `${colors.gray}[${this.timestamp()}]${colors.reset} ${colors.yellow}KVLITE${colors.reset} ${msg}`
    );
  }
}

const logger = new KVLiteLogger(config.kvlite.logging);

/**
 * KVLiteConnection wraps a single TCP socket speaking kvlite's line protocol.
 *
 * IMPORTANT: this class assumes one response line per command. Multi-line
 * responses (KEYS, MGET, HOTKEYS, SCAN, ANOMALIES) will desynchronize the
 * pendingCallbacks queue. Add only single-line commands here, or rework the
 * framing if you need multi-line support.
 */
export class KVLiteConnection extends EventEmitter {
  private socket: net.Socket | null = null;
  private connected: boolean = false;
  private responseBuffer: string = '';
  private pendingCallbacks: Array<(response: string) => void> = [];

  constructor(
    private host: string = config.kvlite.host,
    private port: number = config.kvlite.port
  ) {
    super();
  }

  async connect(): Promise<void> {
    return new Promise((resolve, reject) => {
      this.socket = new net.Socket();

      const timeout = setTimeout(() => {
        this.socket?.destroy();
        reject(new Error('Connection timeout'));
      }, config.kvlite.connectionTimeout);

      this.socket.connect(this.port, this.host, () => {
        clearTimeout(timeout);
        this.connected = true;
        logger.info(`Connected to ${this.host}:${this.port}`);
      });

      this.socket.on('data', (data) => {
        this.responseBuffer += data.toString();
        this.processBuffer();
      });

      this.socket.on('error', (err) => {
        clearTimeout(timeout);
        this.connected = false;
        reject(err);
      });

      this.socket.on('close', () => {
        this.connected = false;
        this.emit('close');
      });

      // Welcome message "+OK kvlite ready\n"
      this.socket.once('data', () => {
        resolve();
      });
    });
  }

  private processBuffer(): void {
    const lines = this.responseBuffer.split('\n');
    this.responseBuffer = lines.pop() || '';

    for (const line of lines) {
      if (line.trim() && this.pendingCallbacks.length > 0) {
        const callback = this.pendingCallbacks.shift();
        callback?.(line.trim());
      }
    }
  }

  async sendCommand(command: string): Promise<string> {
    if (!this.connected || !this.socket) {
      throw new Error('Not connected to kvlite');
    }

    const startTime = performance.now();
    logger.command(command);

    return new Promise((resolve, reject) => {
      const timeout = setTimeout(() => {
        reject(new Error('Command timeout'));
      }, config.kvlite.commandTimeout);

      this.pendingCallbacks.push((response) => {
        clearTimeout(timeout);
        const duration = performance.now() - startTime;
        logger.response(response, duration);
        resolve(response);
      });

      this.socket!.write(command + '\n');
    });
  }

  async close(): Promise<void> {
    if (this.socket && this.connected) {
      try {
        await this.sendCommand('QUIT');
      } catch {
        // Ignore errors on quit
      }
      this.socket.destroy();
      this.socket = null;
      this.connected = false;
    }
  }

  isConnected(): boolean {
    return this.connected;
  }
}

/**
 * KVLitePool manages a small pool of KVLiteConnections so concurrent callers
 * don't share a single socket and its FIFO response queue. Each public method
 * checks out a connection, runs the command, and returns it (even on error).
 */
export class KVLitePool {
  private idle: KVLiteConnection[] = [];
  private waiters: Array<(c: KVLiteConnection) => void> = [];
  private created = 0;
  private closed = false;

  constructor(
    private host: string = config.kvlite.host,
    private port: number = config.kvlite.port,
    private maxConnections: number = config.kvlite.maxConnections
  ) {}

  private async acquire(): Promise<KVLiteConnection> {
    if (this.closed) throw new Error('KVLitePool is closed');

    const idle = this.idle.pop();
    if (idle && idle.isConnected()) return idle;

    if (this.created < this.maxConnections) {
      this.created++;
      try {
        const c = new KVLiteConnection(this.host, this.port);
        await c.connect();
        return c;
      } catch (e) {
        this.created--;
        throw e;
      }
    }

    return new Promise<KVLiteConnection>((resolve) => {
      this.waiters.push(resolve);
    });
  }

  private release(c: KVLiteConnection): void {
    if (this.closed || !c.isConnected()) {
      this.created--;
      // If someone is waiting, give them a fresh connection.
      const w = this.waiters.shift();
      if (w) {
        this.acquire()
          .then(w)
          .catch(() => {
            // Best-effort: a waiter that can't be satisfied is left hanging
            // until another release; in practice acquire() failures here are
            // server-down scenarios and the request will time out.
          });
      }
      return;
    }
    const w = this.waiters.shift();
    if (w) {
      w(c);
    } else {
      this.idle.push(c);
    }
  }

  private async run<T>(fn: (c: KVLiteConnection) => Promise<T>): Promise<T> {
    const c = await this.acquire();
    try {
      return await fn(c);
    } finally {
      this.release(c);
    }
  }

  /**
   * SET key value
   */
  async set(key: string, value: string): Promise<boolean> {
    return this.run(async (c) => (await c.sendCommand(`SET ${key} ${value}`)) === '+OK');
  }

  /**
   * SETEX key seconds value
   */
  async setex(key: string, seconds: number, value: string): Promise<boolean> {
    return this.run(
      async (c) => (await c.sendCommand(`SETEX ${key} ${seconds} ${value}`)) === '+OK'
    );
  }

  /**
   * GET key — returns null if the key doesn't exist
   */
  async get(key: string): Promise<string | null> {
    return this.run(async (c) => {
      const response = await c.sendCommand(`GET ${key}`);
      if (response.startsWith('-ERR')) return null;
      return response;
    });
  }

  /**
   * DELETE key
   */
  async delete(key: string): Promise<boolean> {
    return this.run(async (c) => (await c.sendCommand(`DELETE ${key}`)) === '+OK');
  }

  /**
   * EXISTS key
   */
  async exists(key: string): Promise<boolean> {
    return this.run(async (c) => (await c.sendCommand(`EXISTS ${key}`)) === '1');
  }

  /**
   * INCR key — atomic increment
   */
  async incr(key: string): Promise<number> {
    return this.run(async (c) => {
      const response = await c.sendCommand(`INCR ${key}`);
      if (response.startsWith('-ERR')) throw new Error(response);
      return parseInt(response, 10);
    });
  }

  /**
   * TTL key — seconds remaining, -1 if no TTL, -2 if not found
   */
  async ttl(key: string): Promise<number> {
    return this.run(async (c) => parseInt(await c.sendCommand(`TTL ${key}`), 10));
  }

  /**
   * EXPIRE key seconds
   */
  async expire(key: string, seconds: number): Promise<boolean> {
    return this.run(
      async (c) => (await c.sendCommand(`EXPIRE ${key} ${seconds}`)) === '1'
    );
  }

  /**
   * PING — health check
   */
  async ping(): Promise<boolean> {
    return this.run(async (c) => (await c.sendCommand('PING')) === '+PONG');
  }

  async close(): Promise<void> {
    this.closed = true;
    const conns = this.idle.splice(0);
    await Promise.all(conns.map((c) => c.close().catch(() => undefined)));
  }
}

// Singleton pool reused across the process. Backed by maxConnections sockets.
let poolInstance: KVLitePool | null = null;

export async function getKVLiteClient(): Promise<KVLitePool> {
  if (!poolInstance) {
    poolInstance = new KVLitePool();
  }
  return poolInstance;
}
