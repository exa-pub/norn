import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { ContainerService } from "../gen/norn/server/containers/v1/containers_pb";
import { AgentService } from "../gen/norn/server/agents/v1/agents_pb";
import { TerminalService } from "../gen/norn/server/terminals/v1/terminals_pb";

const transport = createConnectTransport({
  baseUrl: `${window.location.origin}/connect`,
});

export const containerClient = createClient(ContainerService, transport);
export const agentClient = createClient(AgentService, transport);
export const terminalClient = createClient(TerminalService, transport);
