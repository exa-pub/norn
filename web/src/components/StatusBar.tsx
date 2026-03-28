import { Group, Text } from "@mantine/core";
import type { Container } from "../gen/norn/containers/v1/containers_pb";
import { ContainerStatus } from "../gen/norn/containers/v1/containers_pb";

interface StatusBarProps {
  instances: Container[];
  liveAgentCount: number;
  selectedInstance: string | null;
  selectedAgentName: string | null;
}

export function StatusBar({ instances, liveAgentCount, selectedInstance, selectedAgentName }: StatusBarProps) {
  const runningCount = instances.filter((i) => i.status === ContainerStatus.RUNNING).length;

  return (
    <Group
      px="sm"
      h={24}
      justify="space-between"
      bg="dark.8"
      style={{ borderTop: "1px solid var(--mantine-color-dark-4)", flexShrink: 0 }}
    >
      <Group gap="md">
        <Group gap={4}>
          <Text c="green" size="xs">●</Text>
          <Text size="xs" c="dimmed">daemon</Text>
        </Group>
        <Text size="xs" c="dimmed">{instances.length} instances ({runningCount} running)</Text>
        <Text size="xs" c={liveAgentCount > 0 ? "green" : "dimmed"} fw={liveAgentCount > 0 ? 600 : 400}>
          {liveAgentCount} agents live
        </Text>
      </Group>

      <Group gap="xs">
        {selectedInstance && (
          <Text size="xs" c="dimmed">
            {selectedInstance}
            {selectedAgentName && ` / ${selectedAgentName}`}
          </Text>
        )}
      </Group>
    </Group>
  );
}
