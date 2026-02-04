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

/**
 * Logger for kvlite commands
 */
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
 * KVLite TCP Client
 * Implements kvlite's line-based protocol using Node.js net module
 */
export class KVLiteClient extends EventEmitter {
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

  /**
   * Connect to kvlite server
   */
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

      // Handle welcome message "+OK kvlite ready\n"
      this.socket.once('data', () => {
        resolve();
      });
    });
  }

  /**
   * Process response buffer for line-based protocol
   */
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

  /**
   * Send command and wait for response
   */
  private async sendCommand(command: string): Promise<string> {
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

  /**
   * SET key value
   */
  async set(key: string, value: string): Promise<boolean> {
    const response = await this.sendCommand(`SET ${key} ${value}`);
    return response === '+OK';
  }

  /**
   * SETEX key seconds value - Set with TTL
   */
  async setex(key: string, seconds: number, value: string): Promise<boolean> {
    const response = await this.sendCommand(`SETEX ${key} ${seconds} ${value}`);
    return response === '+OK';
  }

  /**
   * GET key
   */
  async get(key: string): Promise<string | null> {
    const response = await this.sendCommand(`GET ${key}`);
    if (response.startsWith('-ERR')) {
      return null;
    }
    return response;
  }

  /**
   * DELETE key
   */
  async delete(key: string): Promise<boolean> {
    const response = await this.sendCommand(`DELETE ${key}`);
    return response === '+OK';
  }

  /**
   * EXISTS key
   */
  async exists(key: string): Promise<boolean> {
    const response = await this.sendCommand(`EXISTS ${key}`);
    return response === '1';
  }

  /**
   * INCR key - Atomic increment
   */
  async incr(key: string): Promise<number> {
    const response = await this.sendCommand(`INCR ${key}`);
    if (response.startsWith('-ERR')) {
      throw new Error(response);
    }
    return parseInt(response, 10);
  }

  /**
   * TTL key - Returns seconds remaining, -1 if no TTL, -2 if not found
   */
  async ttl(key: string): Promise<number> {
    const response = await this.sendCommand(`TTL ${key}`);
    return parseInt(response, 10);
  }

  /**
   * EXPIRE key seconds
   */
  async expire(key: string, seconds: number): Promise<boolean> {
    const response = await this.sendCommand(`EXPIRE ${key} ${seconds}`);
    return response === '1';
  }

  /**
   * PING - Health check
   */
  async ping(): Promise<boolean> {
    const response = await this.sendCommand('PING');
    return response === '+PONG';
  }

  /**
   * Close connection
   */
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

  /**
   * Check connection status
   */
  isConnected(): boolean {
    return this.connected;
  }
}

// Singleton instance
let clientInstance: KVLiteClient | null = null;

export async function getKVLiteClient(): Promise<KVLiteClient> {
  if (!clientInstance || !clientInstance.isConnected()) {
    clientInstance = new KVLiteClient();
    await clientInstance.connect();
  }
  return clientInstance;
}
