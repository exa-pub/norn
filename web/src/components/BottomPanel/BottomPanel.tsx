import { ActionIcon, Box, Group, Loader, Tabs, Text, Tooltip } from "@mantine/core";
import { IconCircleFilled, IconPlus, IconX } from "@tabler/icons-react";
import { useEffect, useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import type { Terminal as TerminalProto } from "../../gen/norn/terminals/v1/terminals_pb";
import type { StreamLogsResponse } from "../../gen/norn/containers/v1/containers_pb";
import { createTerminal, connectWebSocket } from "../../lib/terminal";
import "@xterm/xterm/css/xterm.css";

interface BottomPanelProps {
  terminals: TerminalProto[];
  activeTerminal: string | null;
  creatingTerminal: boolean;
  onSelectTerminal: (id: string) => void;
  onNewTerminal: () => void;
  onCloseTerminal: (id: string) => void;
  // Logs
  logs: StreamLogsResponse[];
  logsActive: boolean;
  activeBottomTab: string;
  onSelectBottomTab: (tab: string) => void;
}

export function BottomPanel({
  terminals,
  activeTerminal,
  creatingTerminal,
  onSelectTerminal,
  onNewTerminal,
  onCloseTerminal,
  logs,
  logsActive,
  activeBottomTab,
  onSelectBottomTab,
}: BottomPanelProps) {
  return (
    <Box style={{ height: "100%", display: "flex", flexDirection: "column", borderTop: "1px solid var(--mantine-color-dark-4)" }}>
      {/* Top-level tabs: Terminals | Logs */}
      <Group px="sm" py={4} justify="space-between" bg="dark.7" style={{ flexShrink: 0 }}>
        <Group gap="xs">
          <Text
            size="xs"
            fw={activeBottomTab === "terminals" ? 700 : 400}
            tt="uppercase"
            c={activeBottomTab === "terminals" ? undefined : "dimmed"}
            style={{ cursor: "pointer" }}
            onClick={() => onSelectBottomTab("terminals")}
          >
            Terminal
          </Text>
          {terminals.length > 0 && (
            <Text size="xs" c="dimmed">{terminals.length}</Text>
          )}
          <Text c="dimmed" size="xs">|</Text>
          <Text
            size="xs"
            fw={activeBottomTab === "logs" ? 700 : 400}
            tt="uppercase"
            c={activeBottomTab === "logs" ? undefined : "dimmed"}
            style={{ cursor: "pointer" }}
            onClick={() => onSelectBottomTab("logs")}
          >
            Logs
          </Text>
          {logsActive && <IconCircleFilled size={8} color="var(--mantine-color-green-6)" />}
        </Group>
        {activeBottomTab === "terminals" && (
          <Tooltip label="New terminal">
            <ActionIcon size="sm" variant="subtle" onClick={onNewTerminal} loading={creatingTerminal}>
              <IconPlus size={14} />
            </ActionIcon>
          </Tooltip>
        )}
      </Group>

      {/* Content */}
      <Box style={{ flex: 1, minHeight: 0, overflow: "hidden" }}>
        {activeBottomTab === "terminals" ? (
          <TerminalsContent
            terminals={terminals}
            activeTerminal={activeTerminal}
            creatingTerminal={creatingTerminal}
            onSelectTerminal={onSelectTerminal}
            onNewTerminal={onNewTerminal}
            onCloseTerminal={onCloseTerminal}
          />
        ) : (
          <LogsContent logs={logs} />
        )}
      </Box>
    </Box>
  );
}

function TerminalsContent({
  terminals,
  activeTerminal,
  creatingTerminal,
  onSelectTerminal,
  onNewTerminal,
  onCloseTerminal,
}: {
  terminals: TerminalProto[];
  activeTerminal: string | null;
  creatingTerminal: boolean;
  onSelectTerminal: (id: string) => void;
  onNewTerminal: () => void;
  onCloseTerminal: (id: string) => void;
}) {
  if (terminals.length === 0) {
    return (
      <Box p="md" h="100%" style={{ display: "flex", alignItems: "center", justifyContent: "center" }}>
        {creatingTerminal ? (
          <Group gap="sm">
            <Loader size="sm" />
            <Text size="sm" c="dimmed">Creating terminal...</Text>
          </Group>
        ) : (
          <Text size="sm" c="dimmed" onClick={onNewTerminal} style={{ cursor: "pointer" }}>
            + New terminal
          </Text>
        )}
      </Box>
    );
  }

  return (
    <Tabs
      value={activeTerminal}
      onChange={(v) => v && onSelectTerminal(v)}
      style={{ height: "100%", display: "flex", flexDirection: "column", overflow: "hidden" }}
      styles={{
        root: { height: "100%", display: "flex", flexDirection: "column", overflow: "hidden" },
        panel: { flex: 1, minHeight: 0, overflow: "hidden" },
      }}
    >
      <Tabs.List>
        {terminals.map((t) => (
          <Tabs.Tab
            key={t.id}
            value={t.id}
            rightSection={
              <IconX
                size={12}
                color="var(--mantine-color-dimmed)"
                style={{ cursor: "pointer" }}
                onClick={(e: React.MouseEvent) => { e.stopPropagation(); onCloseTerminal(t.id); }}
              />
            }
          >
            <Group gap={4}>
              <IconCircleFilled size={8} color="var(--mantine-color-green-6)" />
              <Text size="xs">{t.name || "bash"}</Text>
            </Group>
          </Tabs.Tab>
        ))}
      </Tabs.List>

      {terminals.map((t) => (
        <Tabs.Panel key={t.id} value={t.id} style={{ position: "relative" }}>
          <TerminalView ttyId={t.ttyId} />
        </Tabs.Panel>
      ))}
    </Tabs>
  );
}

function TerminalView({ ttyId }: { ttyId: string }) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!containerRef.current) return;
    containerRef.current.innerHTML = "";

    const handle = createTerminal(containerRef.current);
    const cleanupWs = connectWebSocket(ttyId, handle.terminal, handle.fitAddon);

    return () => {
      cleanupWs();
      handle.dispose();
    };
  }, [ttyId]);

  return <Box ref={containerRef} style={{ position: "absolute", inset: 0 }} />;
}

function LogsContent({ logs }: { logs: StreamLogsResponse[] }) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const stickToBottom = useRef(true);

  const virtualizer = useVirtualizer({
    count: logs.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => 18,
    overscan: 30,
  });

  // Auto-scroll to bottom when new logs arrive (if user hasn't scrolled up)
  useEffect(() => {
    if (stickToBottom.current && logs.length > 0) {
      virtualizer.scrollToIndex(logs.length - 1, { align: "end" });
    }
  }, [logs.length, virtualizer]);

  const handleScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    stickToBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
  };

  if (logs.length === 0) {
    return (
      <Box style={{ height: "100%", padding: "8px 12px", background: "#1e1e1e" }}>
        <Text size="xs" c="dimmed">No logs yet. Logs stream when an instance is selected and running.</Text>
      </Box>
    );
  }

  return (
    <Box
      ref={scrollRef}
      onScroll={handleScroll}
      style={{
        height: "100%",
        overflow: "auto",
        fontFamily: "'JetBrains Mono', 'Fira Code', monospace",
        fontSize: 12,
        lineHeight: 1.5,
        background: "#1e1e1e",
      }}
    >
      <div style={{ height: virtualizer.getTotalSize(), position: "relative" }}>
        {virtualizer.getVirtualItems().map((virtualRow) => {
          const entry = logs[virtualRow.index];
          return (
            <div
              key={virtualRow.index}
              style={{
                position: "absolute",
                top: virtualRow.start,
                left: 0,
                right: 0,
                padding: "0 12px",
                color: entry.isStderr ? "#f87171" : "#d4d4d4",
                whiteSpace: "pre-wrap",
                wordBreak: "break-all",
              }}
            >
              {entry.timestamp && (
                <span style={{ color: "#6b7280", marginRight: 8 }}>
                  {new Date(Number(entry.timestamp.seconds) * 1000).toLocaleTimeString()}
                </span>
              )}
              {entry.line}
            </div>
          );
        })}
      </div>
    </Box>
  );
}
