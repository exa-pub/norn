import { useEffect } from "react";

const FAVICON_SIZE = 32;
const cache = new Map<string, string>();

function createFaviconUrl(color: string): string {
  const cached = cache.get(color);
  if (cached) return cached;

  const canvas = document.createElement("canvas");
  canvas.width = FAVICON_SIZE;
  canvas.height = FAVICON_SIZE;
  const ctx = canvas.getContext("2d")!;
  ctx.beginPath();
  ctx.arc(FAVICON_SIZE / 2, FAVICON_SIZE / 2, FAVICON_SIZE / 2 - 2, 0, Math.PI * 2);
  ctx.fillStyle = color;
  ctx.fill();
  const url = canvas.toDataURL("image/png");
  cache.set(color, url);
  return url;
}

function setFavicon(url: string) {
  let link = document.querySelector<HTMLLinkElement>("link[rel~='icon']");
  if (!link) {
    link = document.createElement("link");
    link.rel = "icon";
    document.head.appendChild(link);
  }
  link.href = url;
}

export function useDocumentTitle(
  selectedInstance: string | null,
  selectedAgentName: string | null,
  liveAgentCount: number,
) {
  useEffect(() => {
    const parts: string[] = [];
    if (selectedAgentName) parts.push(selectedAgentName);
    if (selectedInstance) parts.push(selectedInstance);
    parts.push("Norn");
    document.title = parts.join(" — ");
  }, [selectedInstance, selectedAgentName]);

  useEffect(() => {
    const color = liveAgentCount > 0 ? "#40c057" : "#868e96";
    setFavicon(createFaviconUrl(color));
  }, [liveAgentCount]);
}
