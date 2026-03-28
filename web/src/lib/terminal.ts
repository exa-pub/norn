import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";

export function createTerminal(container: HTMLElement): {
  terminal: Terminal;
  fitAddon: FitAddon;
} {
  const terminal = new Terminal({
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
  terminal.loadAddon(fitAddon);
  terminal.open(container);
  fitAddon.fit();

  return { terminal, fitAddon };
}

export function connectWebSocket(
  ttyId: string,
  terminal: Terminal,
  fitAddon: FitAddon,
  readonly = false
): WebSocket {
  const proto = location.protocol === "https:" ? "wss:" : "ws:";
  const ws = new WebSocket(`${proto}//${location.host}/ws/${ttyId}`);
  ws.binaryType = "arraybuffer";

  ws.onmessage = (e) => {
    terminal.write(new Uint8Array(e.data as ArrayBuffer));
  };

  if (!readonly) {
    terminal.onData((data) => {
      if (ws.readyState === WebSocket.OPEN) ws.send(data);
    });
  }

  terminal.onResize(({ cols, rows }) => {
    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: "resize", cols, rows }));
    }
  });

  ws.onopen = () => fitAddon.fit();

  return ws;
}
