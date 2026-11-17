import { describe, expect, it } from "vitest";

import {
  normalizeGroupOpenAIFast,
  normalizeForcedServiceTier,
  supportsGroupOpenAIFast,
} from "../groupsOpenAIFast";
import en from "@/i18n/locales/en/admin/overview";
import zh from "@/i18n/locales/zh/admin/overview";

describe("groupsOpenAIFast", () => {
  it("supports OpenAI and composite groups", () => {
    expect(supportsGroupOpenAIFast("openai")).toBe(true);
    expect(supportsGroupOpenAIFast("composite")).toBe(true);
    expect(supportsGroupOpenAIFast("anthropic")).toBe(false);
  });

  it("clears stale enabled state on unsupported platforms", () => {
    expect(normalizeGroupOpenAIFast("openai", true)).toBe(true);
    expect(normalizeGroupOpenAIFast("composite", true)).toBe(true);
    expect(normalizeGroupOpenAIFast("anthropic", true)).toBe(false);
    expect(normalizeGroupOpenAIFast("openai", false)).toBe(false);
  });

  it("preserves supported forced tiers and clears tiers on other platforms", () => {
    for (const tier of ["", "default", "priority", "ultrafast"] as const) {
      expect(normalizeForcedServiceTier("openai", tier)).toBe(tier);
      expect(normalizeForcedServiceTier("composite", tier)).toBe(tier);
      expect(normalizeForcedServiceTier("anthropic", tier)).toBe("");
    }
  });

  it("provides localized Ultra Fast tier labels naming the supported models", () => {
    for (const locale of [zh, en]) {
      expect(locale.groups.openaiFast).toMatchObject({
        tierUltrafast: expect.stringContaining("Ultra Fast"),
        hint: expect.stringContaining("service_tier"),
      });
      for (const model of ["gpt-6-astra", "gpt-6.1-sol", "gpt-5.6-sol"]) {
        expect(locale.groups.openaiFast.hint).toContain(model);
      }
    }
  });

  it("provides localized switch labels", () => {
    const labels = {
      title: expect.any(String),
      tier: expect.any(String),
      tierFollow: expect.any(String),
      tierStandard: expect.any(String),
      tierFast: expect.any(String),
      tierUltrafast: expect.any(String),
      hint: expect.stringContaining("service_tier"),
      free: expect.any(String),
      freeHint: expect.stringContaining("Standard"),
      freeUltrafast: expect.any(String),
      freeUltrafastHint: expect.stringContaining("Standard"),
    };
    expect(zh.groups.openaiFast).toMatchObject(labels);
    expect(en.groups.openaiFast).toMatchObject(labels);
  });
});
