import { ActionIcon, Group, Menu, NavLink, ScrollArea, Skeleton, Stack, Text, Tooltip } from "@mantine/core";
import { IconCircleFilled, IconDots, IconEdit, IconLoader2, IconPlayerPlay, IconPlayerStop, IconPlus, IconRobot, IconTrash } from "@tabler/icons-react";
import { ContainerStatus } from "../../gen/norn/containers/v1/containers_pb";
import type { Container } from "../../gen/norn/containers/v1/containers_pb";
import type { AgentSession } from "../../gen/norn/agents/v1/agents_pb";
import { STATUS_COLOR } from "../../lib/status";

interface SidebarProps {
  instances: Container[];
  agents: Map<string, AgentSession[]>;
  selectedInstance: string | null;
  loading?: boolean;
  onSelectInstance: (name: string) => void;
  onSelectAgent: (instanceName: string, agentId: string) => void;
  onNewInstance: () => void;
  onNewAgent: (instanceName: string) => void;
  onStartInstance: (name: string) => void;
  onStopInstance: (name: string) => void;
  onDeleteInstance: (name: string) => void;
  onStopAgent?: (instanceName: string, agentId: string) => void;
  onDeleteAgent?: (instanceName: string, agentId: string) => void;
  onLaunchAgent?: (instanceName: string, agentId: string) => void;
  onRenameAgent?: (instanceName: string, agentId: string) => void;
}

function InstanceMenu({
  inst,
  onStart,
  onStop,
  onDelete,
}: {
  inst: Container;
  onStart: () => void;
  onStop: () => void;
  onDelete: () => void;
}) {
  const isStopped = inst.status === ContainerStatus.STOPPED || inst.status === ContainerStatus.ERROR;
  const isRunning = inst.status === ContainerStatus.RUNNING;

  return (
    <Menu shadow="md" width={160} position="bottom-end" withinPortal>
      <Menu.Target>
        <ActionIcon
          size="xs"
          variant="subtle"
          onClick={(e: React.MouseEvent) => e.stopPropagation()}
        >
          <IconDots size={14} />
        </ActionIcon>
      </Menu.Target>
      <Menu.Dropdown>
        {isStopped && (
          <Menu.Item leftSection={<IconPlayerPlay size={14} />} onClick={onStart}>
            Start
          </Menu.Item>
        )}
        {isRunning && (
          <Menu.Item leftSection={<IconPlayerStop size={14} />} onClick={onStop}>
            Stop
          </Menu.Item>
        )}
        <Menu.Divider />
        <Menu.Item color="red" leftSection={<IconTrash size={14} />} onClick={onDelete}>
          Delete
        </Menu.Item>
      </Menu.Dropdown>
    </Menu>
  );
}

function AgentMenu({
  agent,
  onLaunch,
  onStop,
  onDelete,
  onRename,
}: {
  agent: AgentSession;
  onLaunch?: () => void;
  onStop?: () => void;
  onDelete?: () => void;
  onRename?: () => void;
}) {
  return (
    <Menu shadow="md" width={160} position="bottom-end" withinPortal>
      <Menu.Target>
        <ActionIcon
          size="xs"
          variant="subtle"
          onClick={(e: React.MouseEvent) => e.stopPropagation()}
        >
          <IconDots size={14} />
        </ActionIcon>
      </Menu.Target>
      <Menu.Dropdown>
        {!agent.running && onLaunch && (
          <Menu.Item leftSection={<IconPlayerPlay size={14} />} onClick={onLaunch}>
            Launch
          </Menu.Item>
        )}
        {agent.running && onStop && (
          <Menu.Item leftSection={<IconPlayerStop size={14} />} onClick={onStop}>
            Stop
          </Menu.Item>
        )}
        {onRename && (
          <Menu.Item leftSection={<IconEdit size={14} />} onClick={onRename}>
            Rename
          </Menu.Item>
        )}
        {onDelete && (
          <>
            <Menu.Divider />
            <Menu.Item color="red" leftSection={<IconTrash size={14} />} onClick={onDelete}>
              Delete
            </Menu.Item>
          </>
        )}
      </Menu.Dropdown>
    </Menu>
  );
}

export function Sidebar({
  instances,
  agents,
  selectedInstance,
  loading,
  onSelectInstance,
  onSelectAgent,
  onNewInstance,
  onNewAgent,
  onStartInstance,
  onStopInstance,
  onDeleteInstance,
  onStopAgent,
  onDeleteAgent,
  onLaunchAgent,
  onRenameAgent,
}: SidebarProps) {
  return (
    <Stack gap={0} h="100%">
      <Group px="md" py="xs" justify="space-between">
        <Text size="xs" fw={700} tt="uppercase" c="dimmed">
          Instances
        </Text>
        <Tooltip label="New instance" position="right">
          <ActionIcon size="sm" variant="subtle" onClick={onNewInstance}>
            <IconPlus size={14} />
          </ActionIcon>
        </Tooltip>
      </Group>

      <ScrollArea flex={1}>
        {loading && instances.length === 0 && (
          <Stack gap="xs" px="md" py="xs">
            {[1, 2, 3].map((i) => <Skeleton key={i} height={32} radius="sm" />)}
          </Stack>
        )}
        {instances.map((inst) => {
          const color = STATUS_COLOR[inst.status] ?? "gray";
          const agentList = agents.get(inst.name) ?? [];
          const isSelected = selectedInstance === inst.name;
          const isStarting = inst.status === ContainerStatus.STARTING;

          return (
            <NavLink
              key={inst.name}
              label={
                <Group gap={6} justify="space-between" wrap="nowrap" style={{ flex: 1 }}>
                  <Text size="sm" truncate style={{ flex: 1 }}>{inst.name}</Text>
                  <Group gap={2} wrap="nowrap" onClick={(e: React.MouseEvent) => e.stopPropagation()}>
                    {isStarting && <IconLoader2 size={12} color="var(--mantine-color-yellow-6)" className="spin" />}
                    <InstanceMenu
                      inst={inst}
                      onStart={() => onStartInstance(inst.name)}
                      onStop={() => onStopInstance(inst.name)}
                      onDelete={() => onDeleteInstance(inst.name)}
                    />
                  </Group>
                </Group>
              }
              leftSection={<IconCircleFilled size={8} color={`var(--mantine-color-${color}-6)`} />}
              active={isSelected}
              onClick={() => onSelectInstance(inst.name)}
              opened={isSelected}
              disableRightSectionRotation
            >
              {agentList.map((agent) => (
                <NavLink
                  key={agent.id}
                  label={
                    <Group gap={6} justify="space-between" wrap="nowrap" style={{ flex: 1 }}>
                      <Text size="sm" truncate style={{ flex: 1 }}>{agent.name || agent.id.slice(0, 8)}</Text>
                      <Group gap={2} wrap="nowrap" onClick={(e: React.MouseEvent) => e.stopPropagation()}>
                        <AgentMenu
                          agent={agent}
                          onLaunch={onLaunchAgent ? () => onLaunchAgent(inst.name, agent.id) : undefined}
                          onStop={onStopAgent ? () => onStopAgent(inst.name, agent.id) : undefined}
                          onRename={onRenameAgent ? () => onRenameAgent(inst.name, agent.id) : undefined}
                          onDelete={onDeleteAgent ? () => onDeleteAgent(inst.name, agent.id) : undefined}
                        />
                      </Group>
                    </Group>
                  }
                  leftSection={<IconRobot size={14} color={agent.running ? "var(--mantine-color-green-6)" : undefined} />}
                  onClick={(e: React.MouseEvent) => {
                    e.stopPropagation();
                    onSelectAgent(inst.name, agent.id);
                  }}
                />
              ))}
              <NavLink
                label="New agent session"
                leftSection={<IconPlus size={12} color="var(--mantine-color-dimmed)" />}
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
