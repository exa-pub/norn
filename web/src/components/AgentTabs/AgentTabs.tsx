import { Box, Button, Center, Group, Loader, Stack, Tabs, Text } from "@mantine/core";
import { useEffect, useRef } from "react";
import type { AgentSession } from "../../gen/norn/agents/v1/agents_pb";
import { createTerminal, connectWebSocket } from "../../lib/terminal";
import "@xterm/xterm/css/xterm.css";

interface AgentTabsProps {
  agents: AgentSession[];
  openTabs: string[];
  activeTab: string | null;
  launchingAgents: Set<string>;
  onSelectTab: (agentId: string) => void;
  onCloseTab: (agentId: string) => void;
  onLaunch: (agentId: string) => void;
  onStop: (agentId: string) => void;
}

export function AgentTabs({
  agents,
  openTabs,
  activeTab,
  launchingAgents,
  onSelectTab,
  onCloseTab,
  onLaunch,
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
    <Tabs
      value={activeTab}
      onChange={(v) => v && onSelectTab(v)}
      style={{ height: "100%", display: "flex", flexDirection: "column", overflow: "hidden" }}
      styles={{
        root: { height: "100%", display: "flex", flexDirection: "column", overflow: "hidden" },
        panel: { flex: 1, minHeight: 0, overflow: "hidden", position: "relative" },
      }}
    >
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
                {(agent?.running || launchingAgents.has(id)) && <Text c="green" size="xs">●</Text>}
                <Text size="sm">{agent?.name || id.slice(0, 8)}</Text>
              </Group>
            </Tabs.Tab>
          );
        })}
      </Tabs.List>

      {openTabs.map((id) => {
        const agent = agentMap.get(id);
        const isLaunching = launchingAgents.has(id);
        return (
          <Tabs.Panel key={id} value={id}>
            {agent?.running && agent.ttyId ? (
              <Box style={{ position: "relative", height: "100%" }}>
                <AgentTerminal ttyId={agent.ttyId} />
                <Button
                  size="xs"
                  color="red"
                  variant="light"
                  style={{ position: "absolute", top: 8, right: 8, zIndex: 10 }}
                  onClick={() => onStop(id)}
                >
                  Stop
                </Button>
              </Box>
            ) : isLaunching ? (
              <Center h="100%">
                <Stack align="center" gap="md">
                  <Loader size="md" />
                  <Text c="dimmed" size="sm">Launching agent...</Text>
                </Stack>
              </Center>
            ) : (
              <Center h="100%">
                <Stack align="center" gap="md">
                  <Text c="dimmed" fs="italic">Session idle.</Text>
                  <Text
                    c="blue"
                    size="sm"
                    style={{ cursor: "pointer" }}
                    onClick={() => onLaunch(id)}
                  >
                    Launch
                  </Text>
                </Stack>
              </Center>
            )}
          </Tabs.Panel>
        );
      })}
    </Tabs>
  );
}

function AgentTerminal({ ttyId }: { ttyId: string }) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!containerRef.current) return;
    // Clear any leftover DOM from previous terminal
    containerRef.current.innerHTML = "";

    const handle = createTerminal(containerRef.current);
    const cleanupWs = connectWebSocket(ttyId, handle.terminal, handle.fitAddon);

    return () => {
      cleanupWs();
      handle.dispose();
    };
  }, [ttyId]);

  return (
    <Box
      ref={containerRef}
      style={{ position: "absolute", inset: 0 }}
    />
  );
}
