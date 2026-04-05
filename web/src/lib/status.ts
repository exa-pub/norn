import { ContainerStatus } from "../gen/norn/server/containers/v1/containers_pb";

export const STATUS_COLOR: Record<number, string> = {
  [ContainerStatus.STARTING]: "yellow",
  [ContainerStatus.RUNNING]: "green",
  [ContainerStatus.STOPPED]: "gray",
  [ContainerStatus.ERROR]: "red",
};

export const STATUS_LABEL: Record<number, string> = {
  [ContainerStatus.STARTING]: "Starting",
  [ContainerStatus.RUNNING]: "Running",
  [ContainerStatus.STOPPED]: "Stopped",
  [ContainerStatus.ERROR]: "Error",
};
