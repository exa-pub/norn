import { Box, Button, Center, Group, Stack, Tabs, Text } from "@mantine/core";
import { useEffect, useRef } from "react";
import type { AgentSession } from "../../gen/norn/agents/v1/agents_pb";
import { createTerminal, connectWebSocket } from "../../lib/terminal";
import "@xterm/xterm/css/xterm.css";

interface AgentTabsProps {
  agents: AgentSession[];
  openTabs: string[]; // agent IDs
  activeTab: string | null;
  onSelectTab: (agentId: string) => void;
  onCloseTab: (agentId: string) => void;
  onLaunch: (agentId: string) => void;
  onLaunchWithPrompt: (agentId: string) => void;
  onStop: (agentId: string) => void;
}

export function AgentTabs({
  agents,
  openTabs,
  activeTab,
  onSelectTab,
  onCloseTab,
  onLaunch,
  onLaunchWithPrompt,
  onStop,
}: AgentTabsProps) {
  if (openTabs.length === 0) {
    return (
      <Center h="100%" c="dimmed">
        <Text>Select an agent session from the sidebar</Text>
      </Center>
    );
  }

  const agentMap = new Map(agents.map((a) => [a.id, a]));

  return (
    <Tabs value={activeTab} onChange={(v) => v && onSelectTab(v)} style={{ height: "100%", display: "flex", flexDirection: "column", overflow: "hidden" }}>
      <Tabs.List>
        {openTabs.map((id) => {
          const agent = agentMap.get(id);
          return (
            <Tabs.Tab
              key={id}
              value={id}
              rightSection={
                <Text
                  size="xs"
                  c="dimmed"
                  style={{ cursor: "pointer" }}
                  onClick={(e: React.MouseEvent) => { e.stopPropagation(); onCloseTab(id); }}
                >
                  ×
                </Text>
              }
            >
              <Group gap={4}>
                {agent?.running && <Text c="green" size="xs">●</Text>}
                <Text size="sm">{agent?.name || id.slice(0, 8)}</Text>
              </Group>
            </Tabs.Tab>
          );
        })}
      </Tabs.List>

      {openTabs.map((id) => {
        const agent = agentMap.get(id);
        return (
          <Tabs.Panel key={id} value={id} style={{ flex: 1, minHeight: 0, overflow: "hidden" }}>
            {agent?.running && agent.ttyId ? (
              <AgentTerminal ttyId={agent.ttyId} onStop={() => onStop(id)} />
            ) : (
              <Center h="100%">
                <Stack align="center" gap="md">
                  <Text c="dimmed" fs="italic">Session idle. Launch to resume conversation.</Text>
                  <Group>
                    <Button variant="light" onClick={() => onLaunch(id)}>Launch</Button>
                    <Button variant="subtle" onClick={() => onLaunchWithPrompt(id)}>Launch with prompt...</Button>
                  </Group>
                </Stack>
              </Center>
            )}
          </Tabs.Panel>
        );
      })}
    </Tabs>
  );
}

function AgentTerminal({ ttyId, onStop }: { ttyId: string; onStop: () => void }) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!containerRef.current) return;
    const { terminal, fitAddon } = createTerminal(containerRef.current);
    const ws = connectWebSocket(ttyId, terminal, fitAddon, true); // readonly

    const handleResize = () => fitAddon.fit();
    window.addEventListener("resize", handleResize);

    return () => {
      window.removeEventListener("resize", handleResize);
      ws.close();
      terminal.dispose();
    };
  }, [ttyId]);

  return (
    <Box style={{ height: "100%", position: "relative" }}>
      <Box ref={containerRef} style={{ height: "100%" }} />
      <Button
        size="xs"
        color="red"
        variant="light"
        style={{ position: "absolute", top: 8, right: 8 }}
        onClick={onStop}
      >
        Stop
      </Button>
    </Box>
  );
}
