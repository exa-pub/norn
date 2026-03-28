import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { SearchAddon } from "@xterm/addon-search";
import { Unicode11Addon } from "@xterm/addon-unicode11";
import { WebLinksAddon } from "@xterm/addon-web-links";
import { WebglAddon } from "@xterm/addon-webgl";

export interface TerminalHandle {
  terminal: Terminal;
  fitAddon: FitAddon;
  searchAddon: SearchAddon;
  dispose: () => void;
}

/**
 * Creates an xterm terminal attached to a container element.
 * Uses ResizeObserver for automatic fitting.
 */
export function createTerminal(container: HTMLElement): TerminalHandle {
  const terminal = new Terminal({
    allowProposedApi: true,
    cursorBlink: true,
    fontSize: 13,
    fontFamily: "'JetBrains Mono', 'Fira Code', 'Cascadia Code', monospace",
    theme: {
      background: "#1e1e1e",
      foreground: "#d4d4d4",
      cursor: "#d4d4d4",
    },
  });

  const fitAddon = new FitAddon();
  const searchAddon = new SearchAddon();
  const unicode11Addon = new Unicode11Addon();
  const webLinksAddon = new WebLinksAddon();
  terminal.loadAddon(fitAddon);
  terminal.loadAddon(searchAddon);
  terminal.loadAddon(unicode11Addon);
  terminal.loadAddon(webLinksAddon);
  terminal.open(container);

  // Activate Unicode 11 for correct emoji/CJK width
  terminal.unicode.activeVersion = "11";

  // WebGL renderer for better performance, fallback to default canvas
  try {
    const webglAddon = new WebglAddon();
    webglAddon.onContextLoss(() => webglAddon.dispose());
    terminal.loadAddon(webglAddon);
  } catch {
    // WebGL not available, default renderer is fine
  }

  // Use ResizeObserver for reliable fitting (catches flex/panel resizes, not just window)
  const ro = new ResizeObserver(() => {
    // Only fit if the container has visible dimensions
    if (container.offsetWidth > 0 && container.offsetHeight > 0) {
      try {
        fitAddon.fit();
      } catch {
        // ignore fit errors during teardown
      }
    }
  });
  ro.observe(container);

  // Initial fit deferred to next frame so layout can settle
  requestAnimationFrame(() => {
    try {
      fitAddon.fit();
    } catch {
      // ignore
    }
  });

  const dispose = () => {
    ro.disconnect();
    terminal.dispose();
  };

  return { terminal, fitAddon, searchAddon, dispose };
}

const RECONNECT_BASE_MS = 500;
const RECONNECT_MAX_MS = 8000;

/**
 * Connects a WebSocket to the given ttyId and bridges it to the terminal.
 * Includes auto-reconnect with exponential backoff.
 * Returns a cleanup function.
 */
export function connectWebSocket(
  ttyId: string,
  terminal: Terminal,
  fitAddon: FitAddon,
): () => void {
  let ws: WebSocket | null = null;
  let disposed = false;
  let reconnectAttempt = 0;
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null;

  function connect() {
    if (disposed) return;
    const proto = location.protocol === "https:" ? "wss:" : "ws:";
    ws = new WebSocket(`${proto}//${location.host}/ws/${ttyId}`);
    ws.binaryType = "arraybuffer";

    ws.onopen = () => {
      reconnectAttempt = 0;
      try {
        fitAddon.fit();
      } catch {
        // ignore
      }
      // Always send current size on connect/reconnect so the PTY is in sync
      const { cols, rows } = terminal;
      ws!.send(JSON.stringify({ type: "resize", cols, rows }));
    };

    ws.onmessage = (e) => {
      terminal.write(new Uint8Array(e.data as ArrayBuffer));
    };

    ws.onclose = () => {
      if (disposed) return;
      scheduleReconnect();
    };

    ws.onerror = () => {
      // onclose will fire after onerror, which handles reconnect
    };
  }

  function scheduleReconnect() {
    if (disposed) return;
    const delay = Math.min(RECONNECT_BASE_MS * 2 ** reconnectAttempt, RECONNECT_MAX_MS);
    reconnectAttempt++;
    reconnectTimer = setTimeout(connect, delay);
  }

  // User input → PTY
  const inputDisposable = terminal.onData((data) => {
    if (ws && ws.readyState === WebSocket.OPEN) ws.send(data);
  });

  // Resize events → PTY
  const resizeDisposable = terminal.onResize(({ cols, rows }) => {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: "resize", cols, rows }));
    }
  });

  connect();

  return () => {
    disposed = true;
    if (reconnectTimer) clearTimeout(reconnectTimer);
    inputDisposable.dispose();
    resizeDisposable.dispose();
    if (ws) {
      ws.onclose = null;
      ws.onerror = null;
      ws.close();
    }
  };
}
