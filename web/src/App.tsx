import { AppShell, Box } from "@mantine/core";
import { useCallback, useEffect, useMemo, useState } from "react";
import { Sidebar } from "./components/Sidebar/Sidebar";
import { InstanceHeader } from "./components/InstanceHeader";
import { AgentTabs } from "./components/AgentTabs/AgentTabs";
import { BottomPanel } from "./components/BottomPanel/BottomPanel";
import { StatusBar } from "./components/StatusBar";
import { CreateInstanceModal } from "./components/Modals/CreateInstance";
import { CreateAgentModal } from "./components/Modals/CreateAgent";
import { LaunchAgentModal } from "./components/Modals/LaunchAgent";
import { useInstances } from "./hooks/useInstances";
import { useAgents } from "./hooks/useAgents";
import { useTerminals } from "./hooks/useTerminals";
import { agentClient, terminalClient } from "./client";
import type { AgentSession } from "./gen/norn/agents/v1/agents_pb";

export function App() {
  const instances = useInstances();
  const [selectedInstance, setSelectedInstance] = useState<string | null>(null);
  const agents = useAgents(selectedInstance);
  const terminals = useTerminals(selectedInstance);

  // Per-instance open tabs (preserved across switches)
  const [openAgentTabs, setOpenAgentTabs] = useState<Map<string, string[]>>(new Map());
  const [activeAgentTab, setActiveAgentTab] = useState<Map<string, string>>(new Map());
  const [activeTerminal, setActiveTerminal] = useState<string | null>(null);

  // Modals
  const [createInstanceOpen, setCreateInstanceOpen] = useState(false);
  const [createAgentFor, setCreateAgentFor] = useState<string | null>(null);
  const [launchAgentFor, setLaunchAgentFor] = useState<{ instance: string; session: string } | null>(null);

  // Agents map for sidebar
  const agentsMap = useMemo(() => {
    const m = new Map<string, AgentSession[]>();
    if (selectedInstance) m.set(selectedInstance, agents);
    return m;
  }, [selectedInstance, agents]);

  const currentOpenTabs = selectedInstance ? (openAgentTabs.get(selectedInstance) ?? []) : [];
  const currentActiveTab = selectedInstance ? (activeAgentTab.get(selectedInstance) ?? null) : null;
  const liveAgentCount = agents.filter((a) => a.running).length;

  useEffect(() => {
    if (terminals.length > 0 && !activeTerminal) {
      setActiveTerminal(terminals[0].id);
    }
  }, [terminals, activeTerminal]);

  const openAgentTab = useCallback((instanceName: string, agentId: string) => {
    setSelectedInstance(instanceName);
    setOpenAgentTabs((prev) => {
      const m = new Map(prev);
      const tabs = m.get(instanceName) ?? [];
      if (!tabs.includes(agentId)) m.set(instanceName, [...tabs, agentId]);
      return m;
    });
    setActiveAgentTab((prev) => new Map(prev).set(instanceName, agentId));
  }, []);

  const closeAgentTab = useCallback((agentId: string) => {
    if (!selectedInstance) return;
    setOpenAgentTabs((prev) => {
      const m = new Map(prev);
      m.set(selectedInstance, (m.get(selectedInstance) ?? []).filter((id) => id !== agentId));
      return m;
    });
  }, [selectedInstance]);

  const handleNewTerminal = useCallback(async () => {
    console.log("handleNewTerminal, selectedInstance:", selectedInstance);
    if (!selectedInstance) return;
    try {
      const res = await terminalClient.createTerminal({ instanceName: selectedInstance, name: "" });
      console.log("createTerminal response:", res);
      if (res.terminal) setActiveTerminal(res.terminal.id);
    } catch (e) {
      console.error("createTerminal failed:", e);
    }
  }, [selectedInstance]);

  const handleCloseTerminal = useCallback(async (id: string) => {
    try {
      await terminalClient.closeTerminal({ id });
      if (activeTerminal === id) setActiveTerminal(null);
    } catch { /* ignore */ }
  }, [activeTerminal]);

  const handleLaunch = useCallback(async (agentId: string) => {
    if (!selectedInstance) return;
    try {
      await agentClient.launchAgent({ instanceName: selectedInstance, sessionId: agentId, prompt: "" });
    } catch { /* ignore */ }
  }, [selectedInstance]);

  const handleStop = useCallback(async (agentId: string) => {
    if (!selectedInstance) return;
    try {
      await agentClient.stopAgent({ instanceName: selectedInstance, sessionId: agentId });
    } catch { /* ignore */ }
  }, [selectedInstance]);

  return (
    <Box style={{ height: "100vh", display: "flex", flexDirection: "column", overflow: "hidden" }}>
      <AppShell
        navbar={{ width: 260, breakpoint: 0 }}
        padding={0}
        style={{ flex: 1, minHeight: 0, overflow: "hidden" }}
        styles={{ main: { display: "flex", flexDirection: "column", overflow: "hidden" } }}
      >
        <AppShell.Navbar bg="dark.7">
          <Sidebar
            instances={instances}
            agents={agentsMap}
            selectedInstance={selectedInstance}
            onSelectInstance={setSelectedInstance}
            onSelectAgent={openAgentTab}
            onNewInstance={() => setCreateInstanceOpen(true)}
            onNewAgent={(name) => setCreateAgentFor(name)}
          />
        </AppShell.Navbar>

        <AppShell.Main>
          <InstanceHeader instance={instances.find((i) => i.name === selectedInstance)} />

          <Box style={{ flex: 1, minHeight: 0, overflow: "hidden" }}>
            <AgentTabs
              agents={agents}
              openTabs={currentOpenTabs}
              activeTab={currentActiveTab}
              onSelectTab={(id) =>
                selectedInstance &&
                setActiveAgentTab((prev) => new Map(prev).set(selectedInstance, id))
              }
              onCloseTab={closeAgentTab}
              onLaunch={handleLaunch}
              onLaunchWithPrompt={(id) =>
                selectedInstance && setLaunchAgentFor({ instance: selectedInstance, session: id })
              }
              onStop={handleStop}
            />
          </Box>

          <Box style={{ height: 300, minHeight: 100, flexShrink: 0 }}>
            <BottomPanel
              terminals={terminals}
              activeTerminal={activeTerminal}
              onSelectTerminal={setActiveTerminal}
              onNewTerminal={handleNewTerminal}
              onCloseTerminal={handleCloseTerminal}
            />
          </Box>
        </AppShell.Main>
      </AppShell>

      <StatusBar
        instances={instances}
        liveAgentCount={liveAgentCount}
        selectedInstance={selectedInstance}
        selectedAgent={currentActiveTab}
      />

      <CreateInstanceModal opened={createInstanceOpen} onClose={() => setCreateInstanceOpen(false)} />
      {createAgentFor && (
        <CreateAgentModal opened instanceName={createAgentFor} onClose={() => setCreateAgentFor(null)} />
      )}
      {launchAgentFor && (
        <LaunchAgentModal opened instanceName={launchAgentFor.instance} sessionId={launchAgentFor.session} onClose={() => setLaunchAgentFor(null)} />
      )}
    </Box>
  );
}
