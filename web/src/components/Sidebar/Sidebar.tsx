import { ActionIcon, Badge, Collapse, Group, NavLink, ScrollArea, Stack, Text } from "@mantine/core";
import { ContainerStatus } from "../../gen/norn/containers/v1/containers_pb";
import type { Container } from "../../gen/norn/containers/v1/containers_pb";
import type { AgentSession } from "../../gen/norn/agents/v1/agents_pb";

interface SidebarProps {
  instances: Container[];
  agents: Map<string, AgentSession[]>;
  selectedInstance: string | null;
  onSelectInstance: (name: string) => void;
  onSelectAgent: (instanceName: string, agentId: string) => void;
  onNewInstance: () => void;
  onNewAgent: (instanceName: string) => void;
}

const STATUS_COLOR: Record<number, string> = {
  [ContainerStatus.STARTING]: "yellow",
  [ContainerStatus.RUNNING]: "green",
  [ContainerStatus.STOPPED]: "gray",
  [ContainerStatus.ERROR]: "red",
};

const STATUS_LABEL: Record<number, string> = {
  [ContainerStatus.STARTING]: "Starting",
  [ContainerStatus.RUNNING]: "Running",
  [ContainerStatus.STOPPED]: "Stopped",
  [ContainerStatus.ERROR]: "Error",
};

export function Sidebar({
  instances,
  agents,
  selectedInstance,
  onSelectInstance,
  onSelectAgent,
  onNewInstance,
  onNewAgent,
}: SidebarProps) {
  return (
    <Stack gap={0} h="100%">
      <Group px="md" py="xs" justify="space-between">
        <Text size="xs" fw={700} tt="uppercase" c="dimmed">
          Norn: Instances
        </Text>
        <ActionIcon size="sm" variant="subtle" onClick={onNewInstance}>
          +
        </ActionIcon>
      </Group>

      <ScrollArea flex={1}>
        {instances.map((inst) => {
          const color = STATUS_COLOR[inst.status] ?? "gray";
          const agentList = agents.get(inst.name) ?? [];
          const isSelected = selectedInstance === inst.name;

          return (
            <NavLink
              key={inst.name}
              label={inst.name}
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
