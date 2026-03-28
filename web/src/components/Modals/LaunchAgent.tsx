import { Button, Modal, Textarea } from "@mantine/core";
import { useState } from "react";
import { agentClient } from "../../client";

interface Props {
  opened: boolean;
  instanceName: string;
  sessionId: string;
  onClose: () => void;
}

export function LaunchAgentModal({ opened, instanceName, sessionId, onClose }: Props) {
  const [prompt, setPrompt] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const submit = async () => {
    setLoading(true);
    setError("");
    try {
      await agentClient.launchAgent({ instanceName, sessionId, prompt });
      setPrompt("");
      onClose();
    } catch (e: any) {
      setError(e.message || "Failed to launch");
    } finally {
      setLoading(false);
    }
  };

  return (
    <Modal opened={opened} onClose={onClose} title="Launch Agent">
      <Textarea
        label="Prompt"
        placeholder="What should the agent do?"
        value={prompt}
        onChange={(e) => setPrompt(e.currentTarget.value)}
        error={error}
        minRows={3}
      />
      <Button mt="md" onClick={submit} loading={loading} fullWidth>
        Launch
      </Button>
    </Modal>
  );
}
