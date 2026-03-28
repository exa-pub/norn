import { ActionIcon, Alert, Badge, Button, Group, Text } from "@mantine/core";
import { containerClient } from "../client";
import { ContainerStatus } from "../gen/norn/containers/v1/containers_pb";
import type { Container } from "../gen/norn/containers/v1/containers_pb";

interface Props {
  instance: Container | undefined;
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

export function InstanceHeader({ instance }: Props) {
  if (!instance) return null;

  const color = STATUS_COLOR[instance.status] ?? "gray";
  const label = STATUS_LABEL[instance.status] ?? "Unknown";
  const isStopped = instance.status === ContainerStatus.STOPPED || instance.status === ContainerStatus.ERROR;
  const isRunning = instance.status === ContainerStatus.RUNNING;

  const handleStart = () => containerClient.startContainer({ name: instance.name, removeExisting: true });
  const handleStop = () => containerClient.stopContainer({ name: instance.name });
  const handleDelete = () => {
    if (confirm(`Delete instance "${instance.name}"?`)) {
      containerClient.deleteContainer({ name: instance.name });
    }
  };

  return (
    <div>
      <Group px="md" py={6} bg="dark.7" justify="space-between" style={{ borderBottom: "1px solid var(--mantine-color-dark-4)" }}>
        <Group gap="sm">
          <Text fw={600} size="sm">{instance.name}</Text>
          <Badge size="sm" color={color}>{label}</Badge>
          {instance.dockerId && (
            <Text size="xs" c="dimmed" ff="monospace">{instance.dockerId.slice(0, 12)}</Text>
          )}
        </Group>

        <Group gap="xs">
          {isStopped && (
            <Button size="xs" color="green" variant="light" onClick={handleStart}>
              Start
            </Button>
          )}
          {isRunning && (
            <Button size="xs" color="orange" variant="light" onClick={handleStop}>
              Stop
            </Button>
          )}
          <ActionIcon size="sm" color="red" variant="subtle" onClick={handleDelete}>
            🗑
          </ActionIcon>
        </Group>
      </Group>

      {instance.status === ContainerStatus.ERROR && instance.errorMessage && (
        <Alert color="red" radius={0} py={4} px="md">
          <Text size="xs">{instance.errorMessage}</Text>
        </Alert>
      )}
    </div>
  );
}
