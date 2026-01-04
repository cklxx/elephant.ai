export type SentenceInsight = {
  original: string;
  tightened: string;
  keywords: string[];
  length: number;
  rhythm: "short" | "balanced" | "long";
};

export type DraftStats = {
  characters: number;
  sentences: number;
  paragraphs: number;
  readingMinutes: number;
  keywordCount: number;
};

export type FlowBlocker = {
  id: string;
  title: string;
  detail: string;
  fixHint?: string;
};

const STOP_WORDS = new Set([
  "the",
  "and",
  "for",
  "with",
  "that",
  "this",
  "are",
  "was",
  "were",
  "then",
  "from",
  "into",
  "about",
  "以及",
  "但是",
  "如果",
  "因为",
  "所以",
  "或者",
  "正在",
  "已经",
  "我们",
  "你们",
  "他们",
  "需要",
  "希望",
  "为了",
  "并且",
  "还是",
  "还有",
]);

function normalizeWhitespace(text: string): string {
  return text.replace(/\s+/g, " ").trim();
}

export function splitSentences(text: string): string[] {
  const normalized = text.replace(/\r\n/g, "\n");
  const sentences: string[] = [];
  let buffer = "";

  for (const char of normalized) {
    buffer += char;
    if (/[。！？!?\.]/u.test(char)) {
      const candidate = normalizeWhitespace(buffer);
      if (candidate) {
        sentences.push(candidate);
      }
      buffer = "";
    }
  }

  const tail = normalizeWhitespace(buffer);
  if (tail) {
    sentences.push(tail);
  }

  return sentences;
}

function stripEndingPunctuation(sentence: string): string {
  return sentence.replace(/[。!！?？]+$/u, "").trim();
}

function attachPeriod(sentence: string): string {
  const trimmed = sentence.trim();
  if (!trimmed) return "";
  return /[。!！?？]$/u.test(trimmed) ? trimmed : `${trimmed}。`;
}

function extractConnectors(sentence: string): string[] {
  const connectors = [
    "因此",
    "所以",
    "于是",
    "接着",
    "然后",
    "与此同时",
    "最终",
    "最后",
  ];
  return connectors.filter((keyword) => sentence.includes(keyword));
}

export function tightenSentence(sentence: string): string {
  const normalized = normalizeWhitespace(sentence);
  if (!normalized) return "";

  const cleaned = stripEndingPunctuation(normalized);
  const parts = cleaned
    .split(/[,，;；]/)
    .map((part) => normalizeWhitespace(part))
    .filter(Boolean);

  const head = parts[0] ?? cleaned;
  const tail = parts.slice(1);
  const transitions = ["接着", "然后", "同时", "因此", "最后"];

  const rewritten = [
    head,
    ...tail.map((clause, index) => `${transitions[index % transitions.length]}，${clause}`),
  ].join("，");

  const prefix = extractConnectors(head)[0];
  const merged = prefix && !head.startsWith(prefix) ? `${prefix}，${rewritten}` : rewritten;

  return attachPeriod(merged.replace(/，{2,}/g, "，"));
}

export function extractKeywords(text: string, limit = 8): string[] {
  const tokens = text
    .split(/[\s,.;:!?，。！？；、（）()【】\[\]{}<>“”‘’"'\n]+/)
    .map((token) => token.trim())
    .filter((token) => token.length > 1);

  const unique: string[] = [];
  const seen = new Set<string>();

  for (const token of tokens) {
    const lower = token.toLowerCase();
    if (seen.has(lower)) continue;
    if (STOP_WORDS.has(lower)) continue;
    if (!/[a-zA-Z\u4e00-\u9fff]/u.test(lower)) continue;

    seen.add(lower);
    unique.push(token);
    if (unique.length >= limit) break;
  }

  return unique;
}

export function buildSearchCues(sentences: string[]): string[] {
  const cues = new Set<string>();
  const globalKeywords = extractKeywords(sentences.join(" "), 12);

  if (globalKeywords.length >= 2) {
    cues.add(`${globalKeywords.slice(0, 2).join(" · ")} 背景研究`);
  }

  sentences.forEach((sentence) => {
    const keywords = extractKeywords(sentence, 3);
    if (keywords.length) {
      cues.add(`${keywords.join(" · ")} 相关案例`);
      cues.add(`${keywords[0]} 表述方式`);
    }
    if (sentence.length > 40) {
      cues.add(`${keywords[0] ?? sentence.slice(0, 12)} 精简写法`);
    }
  });

  return Array.from(cues).slice(0, 8);
}

function classifyRhythm(length: number): SentenceInsight["rhythm"] {
  if (length <= 22) return "short";
  if (length <= 48) return "balanced";
  return "long";
}

export function analyzeSentences(sentences: string[]): SentenceInsight[] {
  return sentences.map((sentence) => {
    const tightened = tightenSentence(sentence);
    const keywords = extractKeywords(sentence, 4);
    const length = Array.from(sentence).length;
    return {
      original: sentence,
      tightened,
      keywords,
      length,
      rhythm: classifyRhythm(length),
    };
  });
}

export function buildFlowDraft(sentences: string[]): string {
  if (!sentences.length) return "";

  const cleaned = sentences.map((sentence) => stripEndingPunctuation(sentence)).filter(Boolean);
  if (!cleaned.length) return "";

  const transitions = ["随后", "接着", "同时", "因此", "最后"];
  const chained = cleaned.map((sentence, index) => {
    if (index === 0) return sentence;
    const connector = transitions[(index - 1) % transitions.length];
    return `${connector}，${sentence}`;
  });

  return attachPeriod(chained.join("。"));
}

export function computeDraftStats(
  draft: string,
  sentences: string[],
  keywords: string[],
): DraftStats {
  const characters = draft.replace(/\s+/g, "").length;
  const paragraphs = draft.split(/\n{2,}/).filter((block) => block.trim()).length;
  const estimatedWords = Math.max(1, Math.round(characters / 2));
  const readingMinutes = Math.max(1, Math.ceil(estimatedWords / 220));

  return {
    characters,
    sentences: sentences.length,
    paragraphs,
    readingMinutes,
    keywordCount: keywords.length,
  };
}

export function buildFlowOutline(sentences: string[]): string[] {
  if (!sentences.length) return [];

  const cleaned = sentences.map((sentence) => stripEndingPunctuation(sentence)).filter(Boolean);
  if (!cleaned.length) return [];

  if (cleaned.length === 1) {
    return [cleaned[0]];
  }

  const opening = cleaned[0];
  const closing = cleaned[cleaned.length - 1];
  const middle = cleaned.slice(1, -1);

  const outline: string[] = [
    `开场：${opening}`,
    middle.length
      ? `展开：${middle.join(" / ")}`
      : "展开：补充亮点、论据或步骤",
    `收束：${closing}`,
  ];

  return outline;
}

export function detectBlockers(
  draftText: string,
  sentences: string[],
  keywords: string[],
  paragraphs: number,
): FlowBlocker[] {
  const blockers: FlowBlocker[] = [];
  const normalized = draftText.trim();
  const characterCount = normalized.replace(/\s+/g, "").length;
  const averageSentenceLength =
    sentences.length === 0
      ? 0
      : sentences.reduce((sum, sentence) => sum + Array.from(sentence).length, 0) /
        sentences.length;

  const hasGoalCue = /(目标|目的|想|希望|为了|打算|要做|问题|核心)/.test(normalized);
  const hasAudienceCue = /(受众|读者|客户|老板|同事|团队|你|他|她|他们)/.test(normalized);
  const hasOutcomeCue = /(完成|交付|标准|验收|截止|时间|结果|指标|衡量)/.test(normalized);
  const hasListMarkers = /[-*•]\s+\S+/.test(normalized) || /<(ol|ul)[^>]*>/i.test(normalized);

  if (!hasGoalCue && characterCount > 60) {
    blockers.push({
      id: "unclear-goal",
      title: "核心意图还未收敛",
      detail: "缺少明确的目标/问题表述，容易在铺垫里绕圈。",
      fixHint: "先写一句“我要解决什么”，再展开。",
    });
  }

  if (sentences.length >= 4 && paragraphs <= 1 && !hasListMarkers) {
    blockers.push({
      id: "structure",
      title: "结构信号不足",
      detail: "内容较长但缺少分段/列点，读者难以看出主次顺序。",
      fixHint: "尝试心流重排或转成分点，先铺出骨架。",
    });
  }

  if (averageSentenceLength > 46 || (characterCount > 180 && keywords.length <= 3)) {
    blockers.push({
      id: "expression",
      title: "表述需要压缩",
      detail: "句子偏长或关键词稀疏，可能在堆叙述而缺少判断。",
      fixHint: "用一键紧凑先收短，再补强判断词。",
    });
  }

  if (!hasAudienceCue || !hasOutcomeCue) {
    blockers.push({
      id: "done-definition",
      title: "完成标准不清晰",
      detail: "缺少受众/验收标准，容易在修改中来回推翻。",
      fixHint: "补上“写给谁、看到什么算完成、什么时候要用”。",
    });
  }

  if (characterCount < 120 && keywords.length >= 2 && sentences.length <= 3) {
    blockers.push({
      id: "perfectionism",
      title: "可能在开始阶段卡住",
      detail: "篇幅很短却迟迟未展开，可能在等待完美开头。",
      fixHint: "先随手列 3 个要点或案例，再回头润色。",
    });
  }

  return blockers;
}
