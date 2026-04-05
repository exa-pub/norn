import { Box, Center, Group, Loader, Stack, Tabs, Text } from "@mantine/core";
import { IconCircleFilled, IconX } from "@tabler/icons-react";
import { useCallback, useEffect, useRef, useState } from "react";
import type { AgentSession } from "../../gen/norn/server/agents/v1/agents_pb";
import { createTerminal, connectWebSocket } from "../../lib/terminal";
import "@xterm/xterm/css/xterm.css";

interface AgentTabsProps {
  instanceName: string | null;
  agents: AgentSession[];
  openTabs: string[];
  activeTab: string | null;
  launchingAgents: Set<string>;
  onSelectTab: (agentId: string) => void;
  onCloseTab: (agentId: string) => void;
  onLaunch: (agentId: string) => void;
  onReorderTabs?: (tabs: string[]) => void;
}

export function AgentTabs({
  instanceName,
  agents,
  openTabs,
  activeTab,
  launchingAgents,
  onSelectTab,
  onCloseTab,
  onLaunch,
  onReorderTabs,
}: AgentTabsProps) {
  const dragId = useRef<string | null>(null);
  const [dragOverId, setDragOverId] = useState<string | null>(null);

  const handleDragStart = useCallback((id: string) => {
    dragId.current = id;
  }, []);

  const handleDragOver = useCallback((e: React.DragEvent, id: string) => {
    e.preventDefault();
    setDragOverId(id);
  }, []);

  const handleDrop = useCallback((targetId: string) => {
    const sourceId = dragId.current;
    dragId.current = null;
    setDragOverId(null);
    if (!sourceId || sourceId === targetId || !onReorderTabs) return;
    const newTabs = [...openTabs];
    const fromIdx = newTabs.indexOf(sourceId);
    const toIdx = newTabs.indexOf(targetId);
    if (fromIdx === -1 || toIdx === -1) return;
    newTabs.splice(fromIdx, 1);
    newTabs.splice(toIdx, 0, sourceId);
    onReorderTabs(newTabs);
  }, [openTabs, onReorderTabs]);

  const handleDragEnd = useCallback(() => {
    dragId.current = null;
    setDragOverId(null);
  }, []);
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
              draggable
              onDragStart={() => handleDragStart(id)}
              onDragOver={(e: React.DragEvent) => handleDragOver(e, id)}
              onDrop={() => handleDrop(id)}
              onDragEnd={handleDragEnd}
              style={{ opacity: dragOverId === id ? 0.5 : 1 }}
              rightSection={
                <IconX
                  size={12}
                  color="var(--mantine-color-dimmed)"
                  style={{ cursor: "pointer" }}
                  onClick={(e: React.MouseEvent) => { e.stopPropagation(); onCloseTab(id); }}
                />
              }
            >
              <Group gap={4}>
                {(agent?.running || launchingAgents.has(id)) && <IconCircleFilled size={8} color="var(--mantine-color-green-6)" />}
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
              <Box style={{ position: "absolute", inset: 0, overflow: "hidden", background: "#1e1e1e" }}>
                <AgentTerminal instanceName={instanceName!} ttyId={agent.ttyId} />
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

function AgentTerminal({ instanceName, ttyId }: { instanceName: string; ttyId: string }) {
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!containerRef.current) return;
    // Clear any leftover DOM from previous terminal
    containerRef.current.innerHTML = "";

    const handle = createTerminal(containerRef.current);
    const cleanupWs = connectWebSocket(instanceName, ttyId, handle.terminal, handle.fitAddon);

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
