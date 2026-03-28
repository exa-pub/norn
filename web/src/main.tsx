import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { MantineProvider, createTheme } from "@mantine/core";
import { Notifications } from "@mantine/notifications";
import "@mantine/core/styles.css";
import "@mantine/notifications/styles.css";
import { App } from "./App";
import { initAuth } from "./client/auth";

const theme = createTheme({
  primaryColor: "blue",
  defaultRadius: "sm",
});

// Init auth from URL fragment.
const hasAuth = initAuth();

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <MantineProvider theme={theme} defaultColorScheme="dark">
      <Notifications position="top-right" />
      {hasAuth ? <App /> : <div style={{ padding: 40, color: "#888" }}>Unauthorized. Open the URL with #nornSecret=...</div>}
    </MantineProvider>
  </StrictMode>
);
