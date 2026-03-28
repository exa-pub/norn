import { Group, Menu, NavLink, ScrollArea, Skeleton, Stack, Text, Tooltip, ActionIcon } from "@mantine/core";
import { IconChevronRight, IconCircleFilled, IconEdit, IconLoader2, IconPlayerPlay, IconPlayerStop, IconPlus, IconRobot, IconTrash } from "@tabler/icons-react";
import { useState } from "react";
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

interface ContextMenuState {
  x: number;
  y: number;
  type: "instance" | "agent";
  instanceName: string;
  agent?: AgentSession;
  inst?: Container;
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
  const [ctx, setCtx] = useState<ContextMenuState | null>(null);

  const handleInstanceContext = (e: React.MouseEvent, inst: Container) => {
    e.preventDefault();
    e.stopPropagation();
    setCtx({ x: e.clientX, y: e.clientY, type: "instance", instanceName: inst.name, inst });
  };

  const handleAgentContext = (e: React.MouseEvent, instanceName: string, agent: AgentSession) => {
    e.preventDefault();
    e.stopPropagation();
    setCtx({ x: e.clientX, y: e.clientY, type: "agent", instanceName, agent });
  };

  const isStopped = ctx?.inst && (ctx.inst.status === ContainerStatus.STOPPED || ctx.inst.status === ContainerStatus.ERROR);
  const isRunning = ctx?.inst && ctx.inst.status === ContainerStatus.RUNNING;

  return (
    <Stack gap={0} h="100%">
      <Group px="md" py={8} gap={8} style={{ borderBottom: "1px solid var(--mantine-color-dark-4)" }}>
        <img src="/favicon.svg" alt="Norn" width={20} height={20} />
        <Text size="sm" fw={700}>Norn</Text>
      </Group>

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
              onContextMenu={(e) => handleInstanceContext(e, inst)}
              label={
                <Group gap={6} wrap="nowrap" style={{ flex: 1 }}>
                  <Text size="sm" truncate style={{ flex: 1 }}>{inst.name}</Text>
                  {isStarting && <IconLoader2 size={12} color="var(--mantine-color-yellow-6)" className="spin" />}
                </Group>
              }
              leftSection={
                <Group gap={4} wrap="nowrap">
                  <IconChevronRight
                    size={12}
                    color="var(--mantine-color-dimmed)"
                    style={{
                      transition: "transform 0.2s",
                      transform: isSelected ? "rotate(90deg)" : "rotate(0deg)",
                    }}
                  />
                  <IconCircleFilled size={8} color={`var(--mantine-color-${color}-6)`} />
                </Group>
              }
              active={isSelected}
              onClick={() => onSelectInstance(inst.name)}
              opened={isSelected}
              childrenOffset={0}
              disableRightSectionRotation
              rightSection={null}
            >
              {agentList.map((agent) => (
                <NavLink
                  key={agent.id}
                  style={{ paddingLeft: 28 }}
                  onContextMenu={(e) => handleAgentContext(e, inst.name, agent)}
                  label={
                    <Text size="sm" truncate>{agent.name || agent.id.slice(0, 8)}</Text>
                  }
                  leftSection={<IconRobot size={14} color={agent.running ? "var(--mantine-color-green-6)" : undefined} />}
                  onClick={(e: React.MouseEvent) => {
                    e.stopPropagation();
                    onSelectAgent(inst.name, agent.id);
                  }}
                />
              ))}
              <Text
                size="xs"
                c="dimmed"
                py={6}
                style={{ paddingLeft: 28, cursor: "pointer" }}
                onClick={(e: React.MouseEvent) => {
                  e.stopPropagation();
                  onNewAgent(inst.name);
                }}
              >
                + New session
              </Text>
            </NavLink>
          );
        })}
      </ScrollArea>

      {/* Context menu via Mantine Menu with controlled position */}
      <Menu
        opened={!!ctx}
        onChange={(opened) => { if (!opened) setCtx(null); }}
        position="bottom-start"
        withinPortal
        shadow="md"
        width={180}
      >
        <Menu.Target>
          <div style={{ position: "fixed", left: ctx?.x ?? 0, top: ctx?.y ?? 0, width: 0, height: 0, pointerEvents: "none" }} />
        </Menu.Target>
        <Menu.Dropdown>
          {ctx?.type === "instance" && (
            <>
              {isStopped && (
                <Menu.Item leftSection={<IconPlayerPlay size={14} />} onClick={() => { onStartInstance(ctx.instanceName); setCtx(null); }}>
                  Start
                </Menu.Item>
              )}
              {isRunning && (
                <Menu.Item leftSection={<IconPlayerStop size={14} />} onClick={() => { onStopInstance(ctx.instanceName); setCtx(null); }}>
                  Stop
                </Menu.Item>
              )}
              <Menu.Divider />
              <Menu.Item color="red" leftSection={<IconTrash size={14} />} onClick={() => { onDeleteInstance(ctx.instanceName); setCtx(null); }}>
                Delete
              </Menu.Item>
            </>
          )}
          {ctx?.type === "agent" && ctx.agent && (
            <>
              {!ctx.agent.running && onLaunchAgent && (
                <Menu.Item leftSection={<IconPlayerPlay size={14} />} onClick={() => { onLaunchAgent(ctx.instanceName, ctx.agent!.id); setCtx(null); }}>
                  Launch
                </Menu.Item>
              )}
              {ctx.agent.running && onStopAgent && (
                <Menu.Item leftSection={<IconPlayerStop size={14} />} onClick={() => { onStopAgent(ctx.instanceName, ctx.agent!.id); setCtx(null); }}>
                  Stop
                </Menu.Item>
              )}
              {onRenameAgent && (
                <Menu.Item leftSection={<IconEdit size={14} />} onClick={() => { onRenameAgent(ctx.instanceName, ctx.agent!.id); setCtx(null); }}>
                  Rename
                </Menu.Item>
              )}
              {onDeleteAgent && (
                <>
                  <Menu.Divider />
                  <Menu.Item color="red" leftSection={<IconTrash size={14} />} onClick={() => { onDeleteAgent(ctx.instanceName, ctx.agent!.id); setCtx(null); }}>
                    Delete
                  </Menu.Item>
                </>
              )}
            </>
          )}
        </Menu.Dropdown>
      </Menu>
    </Stack>
  );
}
