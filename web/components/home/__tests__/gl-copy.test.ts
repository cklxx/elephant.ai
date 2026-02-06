import { describe, it, expect } from "vitest";
import { glCopy, type GLHomeCopy } from "../gl/copy";

const LANGS = Object.keys(glCopy) as Array<keyof typeof glCopy>;

describe("glCopy i18n completeness", () => {
  it("has both en and zh languages", () => {
    expect(LANGS).toContain("en");
    expect(LANGS).toContain("zh");
  });

  it("every language has all required keys", () => {
    const requiredKeys: (keyof GLHomeCopy)[] = [
      "title",
      "tagline",
      "cta",
      "ctaHref",
      "keywords",
    ];

    for (const lang of LANGS) {
      for (const key of requiredKeys) {
        expect(glCopy[lang]).toHaveProperty(key);
        expect(glCopy[lang][key]).toBeTruthy();
      }
    }
  });

  it("keywords arrays have the same length across languages", () => {
    const enLen = glCopy.en.keywords.length;
    for (const lang of LANGS) {
      expect(glCopy[lang].keywords.length).toBe(enLen);
    }
  });

  it("keywords are non-empty strings", () => {
    for (const lang of LANGS) {
      for (const keyword of glCopy[lang].keywords) {
        expect(typeof keyword).toBe("string");
        expect(keyword.trim().length).toBeGreaterThan(0);
      }
    }
  });

  it("ctaHref is a valid relative path", () => {
    for (const lang of LANGS) {
      expect(glCopy[lang].ctaHref).toMatch(/^\//);
    }
  });

  it("title is consistent across languages", () => {
    // elephant.ai brand name should be the same in all languages
    expect(glCopy.en.title).toBe(glCopy.zh.title);
  });
});
