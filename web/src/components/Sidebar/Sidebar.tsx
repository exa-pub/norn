import { ActionIcon, Badge, Group, NavLink, ScrollArea, Stack, Text, Tooltip } from "@mantine/core";
import { ContainerStatus } from "../../gen/norn/containers/v1/containers_pb";
import type { Container } from "../../gen/norn/containers/v1/containers_pb";
import type { AgentSession } from "../../gen/norn/agents/v1/agents_pb";
import { STATUS_COLOR } from "../../lib/status";

interface SidebarProps {
  instances: Container[];
  agents: Map<string, AgentSession[]>;
  selectedInstance: string | null;
  onSelectInstance: (name: string) => void;
  onSelectAgent: (instanceName: string, agentId: string) => void;
  onNewInstance: () => void;
  onNewAgent: (instanceName: string) => void;
  onStartInstance: (name: string) => void;
  onStopInstance: (name: string) => void;
  onDeleteInstance: (name: string) => void;
}

export function Sidebar({
  instances,
  agents,
  selectedInstance,
  onSelectInstance,
  onSelectAgent,
  onNewInstance,
  onNewAgent,
  onStartInstance,
  onStopInstance,
  onDeleteInstance,
}: SidebarProps) {
  return (
    <Stack gap={0} h="100%">
      <Group px="md" py="xs" justify="space-between">
        <Text size="xs" fw={700} tt="uppercase" c="dimmed">
          Instances
        </Text>
        <Tooltip label="New instance" position="right">
          <ActionIcon size="sm" variant="subtle" onClick={onNewInstance}>
            +
          </ActionIcon>
        </Tooltip>
      </Group>

      <ScrollArea flex={1}>
        {instances.map((inst) => {
          const color = STATUS_COLOR[inst.status] ?? "gray";
          const agentList = agents.get(inst.name) ?? [];
          const isSelected = selectedInstance === inst.name;
          const isStopped = inst.status === ContainerStatus.STOPPED || inst.status === ContainerStatus.ERROR;
          const isRunning = inst.status === ContainerStatus.RUNNING;
          const isStarting = inst.status === ContainerStatus.STARTING;

          return (
            <NavLink
              key={inst.name}
              label={
                <Group gap={6} justify="space-between" wrap="nowrap" style={{ flex: 1 }}>
                  <Text size="sm" truncate style={{ flex: 1 }}>{inst.name}</Text>
                  <Group gap={2} wrap="nowrap" onClick={(e: React.MouseEvent) => e.stopPropagation()}>
                    {isStopped && (
                      <Tooltip label="Start">
                        <ActionIcon size="xs" variant="subtle" color="green" onClick={() => onStartInstance(inst.name)}>
                          ▶
                        </ActionIcon>
                      </Tooltip>
                    )}
                    {isRunning && (
                      <Tooltip label="Stop">
                        <ActionIcon size="xs" variant="subtle" color="orange" onClick={() => onStopInstance(inst.name)}>
                          ■
                        </ActionIcon>
                      </Tooltip>
                    )}
                    {isStarting && (
                      <Text size="xs" c="yellow">⟳</Text>
                    )}
                    <Tooltip label="Delete">
                      <ActionIcon size="xs" variant="subtle" color="red" onClick={() => onDeleteInstance(inst.name)}>
                        ✕
                      </ActionIcon>
                    </Tooltip>
                  </Group>
                </Group>
              }
              leftSection={<Text c={color} size="xs">●</Text>}
              rightSection={
                agentList.length > 0 ? (
                  <Badge size="xs" variant="filled" color="gray">
                    {agentList.length}
                  </Badge>
                ) : null
              }
              active={isSelected}
              onClick={() => onSelectInstance(inst.name)}
              defaultOpened={isSelected}
            >
              {agentList.map((agent) => (
                <NavLink
                  key={agent.id}
                  label={agent.name || agent.id.slice(0, 8)}
                  leftSection={<Text size="xs">ℛ</Text>}
                  rightSection={
                    <Badge size="xs" color={agent.running ? "green" : "gray"}>
                      {agent.running ? "LIVE" : "IDLE"}
                    </Badge>
                  }
                  onClick={(e: React.MouseEvent) => {
                    e.stopPropagation();
                    onSelectAgent(inst.name, agent.id);
                  }}
                />
              ))}
              <NavLink
                label="New agent session"
                leftSection={<Text size="xs" c="dimmed">+</Text>}
                c="dimmed"
                onClick={(e: React.MouseEvent) => {
                  e.stopPropagation();
                  onNewAgent(inst.name);
                }}
              />
            </NavLink>
          );
        })}
      </ScrollArea>
    </Stack>
  );
}
