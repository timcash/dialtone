type LogLevel = 'INFO' | 'WARN' | 'ERROR';

type LogPayload = {
  level: LogLevel;
  message: string;
  source: string;
  timestamp: string;
};

type NATSPublisher = (subject: string, payload: Uint8Array) => void;

let publisher: NATSPublisher | null = null;
const encoder = new TextEncoder();
const uiLogTopic = 'logs.ui.wsl';

export function setNATSPublisher(next: NATSPublisher | null) {
  publisher = next;
}

function publish(level: LogLevel, source: string, message: string) {
  const payload: LogPayload = {
    level,
    message,
    source,
    timestamp: new Date().toISOString(),
  };
  if (!publisher) return;
  try {
    publisher(uiLogTopic, encoder.encode(JSON.stringify(payload)));
  } catch {
    // keep UI resilient even when NATS publish fails
  }
}

export function logInfo(source: string, message: string) {
  console.log(message);
  publish('INFO', source, message);
}

export function logWarn(source: string, message: string) {
  console.warn(message);
  publish('WARN', source, message);
}

export function logError(source: string, message: string, err?: unknown) {
  if (err !== undefined) {
    console.error(message, err);
  } else {
    console.error(message);
  }
  const full = err === undefined ? message : `${message}: ${errorText(err)}`;
  publish('ERROR', source, full);
}

function errorText(err: unknown): string {
  if (err instanceof Error) return err.message;
  return String(err);
}
