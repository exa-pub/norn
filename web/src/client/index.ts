import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { ContainerService } from "../gen/norn/containers/v1/containers_pb";
import { AgentService } from "../gen/norn/agents/v1/agents_pb";
import { TerminalService } from "../gen/norn/terminals/v1/terminals_pb";

const transport = createConnectTransport({
  baseUrl: window.location.origin,
});

export const containerClient = createClient(ContainerService, transport);
export const agentClient = createClient(AgentService, transport);
export const terminalClient = createClient(TerminalService, transport);
