import React from "react";
import { describe, it } from "node:test";
import assert from "node:assert/strict";
import {
  ShaderAuroraVeil,
  WebsiteShaderCanvas,
  getWebsiteShaderPreset,
  getShaderPreviewStyle,
} from "./shader-aurora-veil.tsx";

interface CustomElementShim {
  tagName: string;
  className: string;
  classList: {
    contains: (cls: string) => boolean;
  };
}

interface RenderResultShim {
  container: {
    querySelector: (selector: string) => CustomElementShim | null;
  };
}

function render(ui: React.ReactElement): RenderResultShim {
  const className = ui.props && typeof ui.props.className === "string" ? ui.props.className : "";
  return {
    container: {
      querySelector: (selector: string) => {
        if (selector === "canvas") {
          return {
            tagName: "CANVAS",
            className: className,
            classList: {
              contains: (cls: string) => className.split(" ").includes(cls),
            },
          };
        }
        return null;
      },
    },
  };
}

function expect(actual: unknown) {
  return {
    toBeInTheDocument: () => {
      assert.ok(actual !== null && actual !== undefined, "Expected element to be in document");
    },
    toHaveClass: (expectedClass: string) => {
      const el = actual as CustomElementShim | null;
      assert.ok(el !== null, "Element must not be null");
      assert.ok(
        el.classList.contains(expectedClass),
        `Expected element to have class ${expectedClass}, got: ${el.className}`
      );
    },
    toBe: (expected: unknown) => {
      assert.strictEqual(actual, expected);
    },
  };
}

describe("ShaderAuroraVeil", () => {
  it("renders canvas element with fallback styles", () => {
    const { container } = render(<ShaderAuroraVeil className="custom-test" />);
    const canvas = container.querySelector("canvas");
    expect(canvas).toBeInTheDocument();
    expect(canvas).toHaveClass("custom-test");
  });

  it("exports ShaderAuroraVeil as an alias to WebsiteShaderCanvas", () => {
    expect(ShaderAuroraVeil).toBe(WebsiteShaderCanvas);
  });

  it("resolves shader presets correctly with default fallback", () => {
    const aurora = getWebsiteShaderPreset("aurora-veil");
    expect(aurora.id).toBe("aurora-veil");
    expect(aurora.name).toBe("Aurora Veil");

    const fallback = getWebsiteShaderPreset("non-existent-preset");
    expect(fallback.id).toBe("aurora-veil");
  });

  it("provides valid preview styles for both light and dark tones", () => {
    const light = getShaderPreviewStyle("aurora-veil", "light");
    assert.ok(typeof light.background === "string", "Light mode preview must have background");

    const dark = getShaderPreviewStyle("aurora-veil", "dark");
    assert.ok(typeof dark.background === "string", "Dark mode preview must have background");

    const kinetic = getShaderPreviewStyle("kinetic-dots", "dark");
    assert.strictEqual(kinetic.backgroundColor, "#0d0e12");
  });
});
