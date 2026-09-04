import { useEffect, useRef } from "react";
import { effect, init, surface } from "vgpu";
import headerGlowSource from "../shaders/header-glow.wgsl";

/**
 * Subtle WebGPU header glow via [vgpu](https://vgpu.sh/).
 * Falls back to the CSS radial gradient when WebGPU is unavailable.
 */
export default function VgpuBackground() {
  const canvasRef = useRef<HTMLCanvasElement>(null);

  useEffect(() => {
    const canvas = canvasRef.current;
    if (!canvas) return;

    let disposed = false;
    let cleanup: (() => void) | undefined;

    const resize = () => {
      const dpr = Math.min(window.devicePixelRatio || 1, 2);
      canvas.width = Math.floor(window.innerWidth * dpr);
      canvas.height = Math.floor(window.innerHeight * dpr);
      canvas.style.width = `${window.innerWidth}px`;
      canvas.style.height = `${window.innerHeight}px`;
    };

    resize();
    window.addEventListener("resize", resize);

    void (async () => {
      try {
        const gpu = await init();
        if (disposed) {
          gpu.dispose();
          return;
        }

        const canvasSurface = surface(gpu, canvas);
        const headerGlow = effect(gpu, headerGlowSource);
        headerGlow.draw(canvasSurface);

        cleanup = () => {
          gpu.dispose();
        };
      } catch {
        canvas.style.display = "none";
      }
    })();

    return () => {
      disposed = true;
      window.removeEventListener("resize", resize);
      cleanup?.();
    };
  }, []);

  return <canvas ref={canvasRef} className="vgpu-background" aria-hidden="true" />;
}
