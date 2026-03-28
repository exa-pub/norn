import { ActionIcon, Box, Group, Tabs, Text } from "@mantine/core";
import { useEffect, useRef } from "react";
import type { Terminal as TerminalProto } from "../../gen/norn/terminals/v1/terminals_pb";
import { createTerminal, connectWebSocket } from "../../lib/terminal";
import "@xterm/xterm/css/xterm.css";

interface BottomPanelProps {
  terminals: TerminalProto[];
  activeTerminal: string | null;
  onSelectTerminal: (id: string) => void;
  onNewTerminal: () => void;
  onCloseTerminal: (id: string) => void;
}

export function BottomPanel({
  terminals,
  activeTerminal,
  onSelectTerminal,
  onNewTerminal,
  onCloseTerminal,
}: BottomPanelProps) {
  return (
    <Box style={{ height: "100%", display: "flex", flexDirection: "column", borderTop: "1px solid var(--mantine-color-dark-4)" }}>
      <Group px="sm" py={4} justify="space-between" bg="dark.7">
        <Group gap="xs">
          <Text size="xs" fw={600} tt="uppercase">Terminal</Text>
          {terminals.length > 0 && (
            <Text size="xs" c="dimmed">{terminals.length}</Text>
          )}
        </Group>
      </Group>

      {terminals.length === 0 ? (
        <Box p="md">
          <Text size="sm" c="dimmed" onClick={onNewTerminal} style={{ cursor: "pointer" }}>
            + New terminal
          </Text>
        </Box>
      ) : (
        <Tabs
          value={activeTerminal}
          onChange={(v) => v && onSelectTerminal(v)}
          style={{ flex: 1, display: "flex", flexDirection: "column" }}
        >
          <Tabs.List>
            {terminals.map((t) => (
              <Tabs.Tab
                key={t.id}
                value={t.id}
                rightSection={
                  <Text
                    size="xs"
                    c="dimmed"
                    style={{ cursor: "pointer" }}
                    onClick={(e: React.MouseEvent) => { e.stopPropagation(); onCloseTerminal(t.id); }}
                  >
                    ×
                  </Text>
                }
              >
                <Group gap={4}>
                  <Text c="green" size="xs">●</Text>
                  <Text size="xs">{t.name || "bash"}</Text>
                </Group>
              </Tabs.Tab>
            ))}
            <ActionIcon size="sm" variant="subtle" ml={4} onClick={onNewTerminal}>
              +
            </ActionIcon>
          </Tabs.List>

          {terminals.map((t) => (
            <Tabs.Panel key={t.id} value={t.id} style={{ flex: 1 }}>
              <TerminalView ttyId={t.ttyId} />
            </Tabs.Panel>
          ))}
        </Tabs>
      )}
    </Box>
  );
}

function TerminalView({ ttyId }: { ttyId: string }) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!containerRef.current) return;
    const { terminal, fitAddon } = createTerminal(containerRef.current);
    const ws = connectWebSocket(ttyId, terminal, fitAddon);

    const handleResize = () => fitAddon.fit();
    window.addEventListener("resize", handleResize);

    return () => {
      window.removeEventListener("resize", handleResize);
      ws.close();
      terminal.dispose();
    };
  }, [ttyId]);

  return <Box ref={containerRef} style={{ height: "100%" }} />;
}
