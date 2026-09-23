"use client";

import { cn } from "@/lib/utils";
import React, { useEffect, useRef, useState } from "react";

export type WebsiteShaderPresetId = "aurora-veil" | "kinetic-dots";

export interface WebsiteShaderPreset {
  id: WebsiteShaderPresetId;
  name: string;
  description: string;
}

export const websiteShaderPresets: Record<WebsiteShaderPresetId, WebsiteShaderPreset> = {
  "aurora-veil": {
    id: "aurora-veil",
    name: "Aurora Veil",
    description: "Atmospheric sinusoidal veil with organic flowing curtains",
  },
  "kinetic-dots": {
    id: "kinetic-dots",
    name: "Kinetic Dots",
    description: "High-precision interactive grid of pulsating kinetic particles",
  },
};

export function getWebsiteShaderPreset(id: string): WebsiteShaderPreset {
  if (id in websiteShaderPresets) {
    return websiteShaderPresets[id as WebsiteShaderPresetId];
  }
  return websiteShaderPresets["aurora-veil"];
}

export function getShaderPreviewStyle(
  preset: string,
  tone: "light" | "dark" = "light"
): React.CSSProperties {
  if (preset === "kinetic-dots") {
    return {
      backgroundColor: tone === "dark" ? "#0d0e12" : "#faf9f6",
      backgroundImage:
        tone === "dark"
          ? "radial-gradient(#262a33 1px, transparent 1px)"
          : "radial-gradient(#d1d5db 1px, transparent 1px)",
      backgroundSize: "24px 24px",
    };
  }

  return {
    background:
      tone === "dark"
        ? "radial-gradient(ellipse at 50% 30%, #172033 0%, #0a0c10 70%)"
        : "radial-gradient(ellipse at 50% 30%, #e8f0fe 0%, #faf9f6 70%)",
  };
}

export function useReducedMotion(): boolean {
  const [matches, setMatches] = useState(() => {
    if (typeof window === "undefined") return false;
    return window.matchMedia("(prefers-reduced-motion: reduce)").matches;
  });

  useEffect(() => {
    if (typeof window === "undefined") return;
    const mql = window.matchMedia("(prefers-reduced-motion: reduce)");
    const handler = (e: MediaQueryListEvent) => setMatches(e.matches);
    mql.addEventListener("change", handler);
    return () => mql.removeEventListener("change", handler);
  }, []);

  return matches;
}

function useThemeTone(explicitTone?: "light" | "dark" | "auto"): "light" | "dark" {
  const [detectedTone, setDetectedTone] = useState<"light" | "dark">(() => {
    if (typeof window === "undefined") return "light";
    const isDark =
      document.documentElement.classList.contains("dark") ||
      document.body.classList.contains("dark") ||
      window.matchMedia("(prefers-color-scheme: dark)").matches;
    return isDark ? "dark" : "light";
  });

  useEffect(() => {
    if (typeof window === "undefined") return;
    if (explicitTone === "light" || explicitTone === "dark") return;

    const update = () => {
      const isDark =
        document.documentElement.classList.contains("dark") ||
        document.body.classList.contains("dark") ||
        window.matchMedia("(prefers-color-scheme: dark)").matches;
      setDetectedTone(isDark ? "dark" : "light");
    };

    const observer = new MutationObserver(update);
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["class"],
    });

    const mql = window.matchMedia("(prefers-color-scheme: dark)");
    mql.addEventListener("change", update);

    return () => {
      observer.disconnect();
      mql.removeEventListener("change", update);
    };
  }, [explicitTone]);

  if (explicitTone === "light" || explicitTone === "dark") {
    return explicitTone;
  }

  return detectedTone;
}

const VERTEX_SHADER_SOURCE = `
attribute vec2 a_position;
void main() {
  gl_Position = vec4(a_position, 0.0, 1.0);
}
`;

const FRAGMENT_SHADER_AURORA = `
precision mediump float;
uniform vec2 u_resolution;
uniform float u_time;
uniform vec2 u_mouse;
uniform float u_dark;

void main() {
  vec2 st = gl_FragCoord.xy / u_resolution.xy;
  st.y = 1.0 - st.y;
  float aspect = u_resolution.x / max(u_resolution.y, 1.0);
  vec2 uv = st;
  uv.x *= aspect;

  vec2 mouse = u_mouse * vec2(aspect, 1.0);
  float distToMouse = length(uv - mouse);
  float mouseWave = smoothstep(0.8, 0.0, distToMouse) * 0.15;

  float t = u_time * 0.25;
  float wave1 = sin(uv.x * 2.2 + t + sin(uv.y * 3.1 + t * 0.6));
  float wave2 = cos(uv.x * 3.4 - t * 0.7 + cos(uv.y * 2.4 + t * 0.8));
  float wave3 = sin((uv.x + uv.y) * 2.8 + t * 0.45);

  float veil = smoothstep(0.15, 0.85, (wave1 + wave2 + wave3) / 3.0 + mouseWave);

  vec3 bgLight = vec3(0.98, 0.976, 0.965);
  vec3 veilLight = vec3(0.90, 0.925, 0.91);
  vec3 accentLight = vec3(0.92, 0.94, 0.965);

  vec3 bgDark = vec3(0.051, 0.055, 0.067);
  vec3 veilDark = vec3(0.094, 0.133, 0.173);
  vec3 accentDark = vec3(0.078, 0.110, 0.149);

  vec3 bg = mix(bgLight, bgDark, u_dark);
  vec3 veilCol = mix(veilLight, veilDark, u_dark);
  vec3 accentCol = mix(accentLight, accentDark, u_dark);

  vec3 color = mix(bg, veilCol, veil * 0.5);
  color = mix(color, accentCol, smoothstep(0.25, 0.75, wave2) * 0.3);

  gl_FragColor = vec4(color, 1.0);
}
`;

const FRAGMENT_SHADER_KINETIC_DOTS = `
precision mediump float;
uniform vec2 u_resolution;
uniform float u_time;
uniform vec2 u_mouse;
uniform float u_dark;

void main() {
  vec2 st = gl_FragCoord.xy / u_resolution.xy;
  st.y = 1.0 - st.y;
  float aspect = u_resolution.x / max(u_resolution.y, 1.0);
  vec2 uv = st;
  uv.x *= aspect;

  vec2 mouse = u_mouse * vec2(aspect, 1.0);
  float distToMouse = length(uv - mouse);
  float mousePulse = sin(distToMouse * 14.0 - u_time * 2.5) * exp(-distToMouse * 2.8);

  float gridSize = 32.0;
  vec2 gridUv = fract(uv * gridSize) - 0.5;
  float d = length(gridUv);

  float t = u_time * 1.2;
  float wave = sin(t + uv.x * 3.0 + uv.y * 3.0) * 0.5 + 0.5;
  float dotRadius = 0.08 + 0.05 * wave + 0.07 * mousePulse;
  float dot = smoothstep(dotRadius, dotRadius - 0.03, d);

  vec3 bgLight = vec3(0.975, 0.973, 0.968);
  vec3 dotLight = vec3(0.74, 0.76, 0.79);

  vec3 bgDark = vec3(0.051, 0.055, 0.067);
  vec3 dotDark = vec3(0.22, 0.25, 0.29);

  vec3 bg = mix(bgLight, bgDark, u_dark);
  vec3 dotCol = mix(dotLight, dotDark, u_dark);

  vec3 color = mix(bg, dotCol, dot * 0.65);
  gl_FragColor = vec4(color, 1.0);
}
`;

function createShader(gl: WebGLRenderingContext, type: number, source: string): WebGLShader | null {
  const shader = gl.createShader(type);
  if (!shader) return null;
  gl.shaderSource(shader, source);
  gl.compileShader(shader);
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    gl.deleteShader(shader);
    return null;
  }
  return shader;
}

function createProgram(
  gl: WebGLRenderingContext,
  vertexShader: WebGLShader,
  fragmentShader: WebGLShader
): WebGLProgram | null {
  const program = gl.createProgram();
  if (!program) return null;
  gl.attachShader(program, vertexShader);
  gl.attachShader(program, fragmentShader);
  gl.linkProgram(program);
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
    gl.deleteProgram(program);
    return null;
  }
  return program;
}

export interface WebsiteShaderCanvasProps {
  preset?: WebsiteShaderPresetId | string;
  tone?: "light" | "dark" | "auto";
  speed?: number;
  className?: string;
  style?: React.CSSProperties;
  interactive?: boolean;
}

export function WebsiteShaderCanvas({
  preset = "aurora-veil",
  tone = "auto",
  speed = 1.0,
  className,
  style,
  interactive = true,
}: WebsiteShaderCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const containerRef = useRef<HTMLDivElement | null>(null);
  const [webglFailed, setWebglFailed] = useState(false);
  const prefersReducedMotion = useReducedMotion();
  const effectiveTone = useThemeTone(tone);

  useEffect(() => {
    if (webglFailed || prefersReducedMotion) return;

    const canvas = canvasRef.current;
    const container = containerRef.current;
    if (!canvas || !container) return;

    const gl =
      canvas.getContext("webgl") ||
      (canvas.getContext("experimental-webgl") as WebGLRenderingContext | null);

    if (!gl) {
      setWebglFailed(true);
      return;
    }

    const fragmentSource =
      preset === "kinetic-dots" ? FRAGMENT_SHADER_KINETIC_DOTS : FRAGMENT_SHADER_AURORA;

    const vShader = createShader(gl, gl.VERTEX_SHADER, VERTEX_SHADER_SOURCE);
    const fShader = createShader(gl, gl.FRAGMENT_SHADER, fragmentSource);

    if (!vShader || !fShader) {
      setWebglFailed(true);
      return;
    }

    const program = createProgram(gl, vShader, fShader);
    if (!program) {
      setWebglFailed(true);
      return;
    }

    const positionBuffer = gl.createBuffer();
    gl.bindBuffer(gl.ARRAY_BUFFER, positionBuffer);
    const positions = new Float32Array([-1, -1, 1, -1, -1, 1, -1, 1, 1, -1, 1, 1]);
    gl.bufferData(gl.ARRAY_BUFFER, positions, gl.STATIC_DRAW);

    const posLoc = gl.getAttribLocation(program, "a_position");
    const resLoc = gl.getUniformLocation(program, "u_resolution");
    const timeLoc = gl.getUniformLocation(program, "u_time");
    const mouseLoc = gl.getUniformLocation(program, "u_mouse");
    const darkLoc = gl.getUniformLocation(program, "u_dark");

    let animationFrameId = 0;
    let startTime = performance.now();
    let currentMouseX = 0.5;
    let currentMouseY = 0.5;
    let targetMouseX = 0.5;
    let targetMouseY = 0.5;

    function resize() {
      if (!canvas || !container || !gl) return;
      const rect = container.getBoundingClientRect();
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      const width = Math.max(1, Math.floor(rect.width * dpr));
      const height = Math.max(1, Math.floor(rect.height * dpr));

      if (canvas.width !== width || canvas.height !== height) {
        canvas.width = width;
        canvas.height = height;
        gl.viewport(0, 0, width, height);
      }
    }

    resize();
    const resizeObserver = new ResizeObserver(() => resize());
    resizeObserver.observe(container);

    function handlePointerMove(e: PointerEvent) {
      if (!interactive || !container) return;
      const rect = container.getBoundingClientRect();
      if (rect.width > 0 && rect.height > 0) {
        targetMouseX = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
        targetMouseY = Math.max(0, Math.min(1, 1.0 - (e.clientY - rect.top) / rect.height));
      }
    }

    if (interactive) {
      window.addEventListener("pointermove", handlePointerMove);
    }

    function render(now: number) {
      if (!gl || !program || !canvas) return;

      currentMouseX += (targetMouseX - currentMouseX) * 0.08;
      currentMouseY += (targetMouseY - currentMouseY) * 0.08;

      const elapsed = ((now - startTime) / 1000) * speed;

      gl.useProgram(program);

      gl.bindBuffer(gl.ARRAY_BUFFER, positionBuffer);
      gl.enableVertexAttribArray(posLoc);
      gl.vertexAttribPointer(posLoc, 2, gl.FLOAT, false, 0, 0);

      if (resLoc) gl.uniform2f(resLoc, canvas.width, canvas.height);
      if (timeLoc) gl.uniform1f(timeLoc, elapsed);
      if (mouseLoc) gl.uniform2f(mouseLoc, currentMouseX, currentMouseY);
      if (darkLoc) gl.uniform1f(darkLoc, effectiveTone === "dark" ? 1.0 : 0.0);

      gl.drawArrays(gl.TRIANGLES, 0, 6);

      animationFrameId = requestAnimationFrame(render);
    }

    animationFrameId = requestAnimationFrame(render);

    function handleContextLost(e: Event) {
      e.preventDefault();
      setWebglFailed(true);
    }

    canvas.addEventListener("webglcontextlost", handleContextLost);

    return () => {
      cancelAnimationFrame(animationFrameId);
      resizeObserver.disconnect();
      if (interactive) {
        window.removeEventListener("pointermove", handlePointerMove);
      }
      canvas.removeEventListener("webglcontextlost", handleContextLost);

      if (gl) {
        gl.deleteBuffer(positionBuffer);
        gl.deleteShader(vShader);
        gl.deleteShader(fShader);
        gl.deleteProgram(program);
      }
    };
  }, [preset, speed, effectiveTone, webglFailed, prefersReducedMotion, interactive]);

  const fallbackStyle = getShaderPreviewStyle(preset, effectiveTone);

  return (
    <div
      ref={containerRef}
      className={cn("relative h-full w-full overflow-hidden", className)}
      style={{ ...fallbackStyle, ...style }}
    >
      {!webglFailed && !prefersReducedMotion ? (
        <canvas ref={canvasRef} className="absolute inset-0 h-full w-full pointer-events-none" />
      ) : null}
    </div>
  );
}

export function WebsiteShaderDemo({ className }: { className?: string }) {
  const [selectedPreset, setSelectedPreset] = useState<WebsiteShaderPresetId>("aurora-veil");
  const [selectedTone, setSelectedTone] = useState<"auto" | "light" | "dark">("auto");

  return (
    <div
      className={cn(
        "relative overflow-hidden rounded-xl border border-border bg-card p-4",
        className
      )}
    >
      <div className="relative h-64 w-full overflow-hidden rounded-lg border border-border">
        <WebsiteShaderCanvas
          preset={selectedPreset}
          tone={selectedTone}
          className="h-full w-full"
        />
      </div>
      <div className="mt-4 flex flex-wrap items-center justify-between gap-2 text-xs">
        <div className="flex items-center gap-1.5 font-mono">
          <span className="text-muted-foreground">Preset:</span>
          {(Object.keys(websiteShaderPresets) as WebsiteShaderPresetId[]).map((key) => (
            <button
              key={key}
              type="button"
              onClick={() => setSelectedPreset(key)}
              className={cn(
                "rounded px-2 py-1 transition-colors",
                selectedPreset === key
                  ? "bg-foreground text-background"
                  : "bg-muted text-foreground hover:bg-muted/80"
              )}
            >
              {websiteShaderPresets[key].name}
            </button>
          ))}
        </div>
        <div className="flex items-center gap-1.5 font-mono">
          <span className="text-muted-foreground">Tone:</span>
          {(["auto", "light", "dark"] as const).map((t) => (
            <button
              key={t}
              type="button"
              onClick={() => setSelectedTone(t)}
              className={cn(
                "rounded px-2 py-1 transition-colors uppercase",
                selectedTone === t
                  ? "bg-foreground text-background"
                  : "bg-muted text-foreground hover:bg-muted/80"
              )}
            >
              {t}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

export const ShaderAuroraVeil = WebsiteShaderCanvas;
export default WebsiteShaderCanvas;
