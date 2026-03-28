import { useEffect } from "react";

const FAVICON_SIZE = 64;
const DOT_RADIUS = 10;
const cache = new Map<string, string>();

function createFaviconUrl(color: string): string {
  const cached = cache.get(color);
  if (cached) return cached;

  const canvas = document.createElement("canvas");
  canvas.width = FAVICON_SIZE;
  canvas.height = FAVICON_SIZE;
  const ctx = canvas.getContext("2d")!;

  const r = 14;
  ctx.beginPath();
  ctx.moveTo(r, 0);
  ctx.lineTo(FAVICON_SIZE - r, 0);
  ctx.quadraticCurveTo(FAVICON_SIZE, 0, FAVICON_SIZE, r);
  ctx.lineTo(FAVICON_SIZE, FAVICON_SIZE - r);
  ctx.quadraticCurveTo(FAVICON_SIZE, FAVICON_SIZE, FAVICON_SIZE - r, FAVICON_SIZE);
  ctx.lineTo(r, FAVICON_SIZE);
  ctx.quadraticCurveTo(0, FAVICON_SIZE, 0, FAVICON_SIZE - r);
  ctx.lineTo(0, r);
  ctx.quadraticCurveTo(0, 0, r, 0);
  ctx.closePath();
  ctx.fillStyle = "#0F0F0F";
  ctx.fill();

  ctx.font = "bold 38px 'Arial Black', sans-serif";
  ctx.textAlign = "center";
  ctx.textBaseline = "middle";
  ctx.fillStyle = "#FF2D55";
  ctx.strokeStyle = "#0F0F0F";
  ctx.lineWidth = 2;
  ctx.strokeText("{}", FAVICON_SIZE / 2, FAVICON_SIZE / 2);
  ctx.fillText("{}", FAVICON_SIZE / 2, FAVICON_SIZE / 2);

  ctx.beginPath();
  ctx.arc(FAVICON_SIZE - DOT_RADIUS - 1, FAVICON_SIZE - DOT_RADIUS - 1, DOT_RADIUS, 0, Math.PI * 2);
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
