"use client";

import { useEffect, useMemo, useState } from "react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { toast } from "@/components/ui/toast";
import { getSessionPersona, updateSessionPersona } from "@/lib/api";
import type { UserPersonaDrive, UserPersonaProfile } from "@/lib/types";
import { cn } from "@/lib/utils";

const driveOptions = [
  { id: "autonomy", label: "Autonomy / agency" },
  { id: "competence", label: "Competence / mastery" },
  { id: "relatedness", label: "Relatedness / belonging" },
  { id: "meaning", label: "Meaning / purpose" },
  { id: "security", label: "Security / stability" },
  { id: "novelty", label: "Novelty / exploration" },
] as const;

type DriveKey = (typeof driveOptions)[number]["id"];

const traitOptions = [
  { id: "openness", label: "Openness" },
  { id: "conscientiousness", label: "Conscientiousness" },
  { id: "extraversion", label: "Extraversion" },
  { id: "agreeableness", label: "Agreeableness" },
  { id: "neuroticism", label: "Neuroticism" },
] as const;

type TraitKey = (typeof traitOptions)[number]["id"];

const initiativeOptions = [
  "Curiosity and learning",
  "Responsibility to people I care about",
  "External deadlines / accountability",
  "Competition or status signals",
  "Avoiding loss or regret",
  "Impact and contribution",
];

const constructionRules = [
  "Rank core drives by score; the top two are treated as primary drives.",
  "Decision style is derived from speed-vs-quality and evidence preference answers.",
  "Risk profile and conflict style are derived from the key choice defaults.",
  "Non-negotiables always override other preferences when conflicts arise.",
];

const defaultDriveScores = driveOptions.reduce<Record<DriveKey, number>>(
  (acc, item) => {
    acc[item.id] = 3;
    return acc;
  },
  {} as Record<DriveKey, number>,
);

const defaultTraits = traitOptions.reduce<Record<TraitKey, number>>(
  (acc, item) => {
    acc[item.id] = 4;
    return acc;
  },
  {} as Record<TraitKey, number>,
);

function buildSummary(profile: UserPersonaProfile) {
  const drives = profile.top_drives?.join(", ") ?? "";
  const values = profile.values?.join(", ") ?? "";
  const initiative = profile.initiative_sources?.join(", ") ?? "";
  const focus = profile.goals?.current_focus ?? "";

  const parts = [
    drives ? `Primary drives: ${drives}.` : "",
    initiative ? `Initiative sources: ${initiative}.` : "",
    values ? `Core values: ${values}.` : "",
    focus ? `Current focus: ${focus}.` : "",
    profile.decision_style ? `Decision style: ${profile.decision_style}.` : "",
    profile.risk_profile ? `Risk profile: ${profile.risk_profile}.` : "",
  ].filter(Boolean);

  return parts.join(" ");
}

function toDriveEntries(scores: Record<DriveKey, number>): UserPersonaDrive[] {
  return driveOptions.map((item) => ({
    id: item.id,
    label: item.label,
    score: scores[item.id],
  }));
}

function deriveDecisionStyle(speedQuality: string, evidenceStyle: string) {
  const speedLabel =
    speedQuality === "speed"
      ? "speed-first with iteration"
      : speedQuality === "quality"
        ? "quality-first with careful validation"
        : "balanced speed and quality";
  const evidenceLabel =
    evidenceStyle === "data"
      ? "data-first"
      : evidenceStyle === "intuition"
        ? "intuition-guided"
        : "balanced data and intuition";
  return `${speedLabel}, ${evidenceLabel}`;
}

function deriveKeyChoices(
  speedQuality: string,
  riskPreference: string,
  peoplePreference: string,
) {
  const speedLabel =
    speedQuality === "speed"
      ? "When speed conflicts with quality, I bias toward speed and learn fast."
      : speedQuality === "quality"
        ? "When speed conflicts with quality, I bias toward quality and durability."
        : "When speed conflicts with quality, I keep a balanced trade-off.";
  const riskLabel =
    riskPreference === "bold"
      ? "When outcomes are uncertain, I accept bold bets for upside."
      : riskPreference === "conservative"
        ? "When outcomes are uncertain, I favor stability and downside protection."
        : "When outcomes are uncertain, I balance upside and downside.";
  const peopleLabel =
    peoplePreference === "principle"
      ? "When goals conflict with relationships, I prioritize principles over harmony."
      : peoplePreference === "harmony"
        ? "When goals conflict with relationships, I prioritize harmony and trust."
        : "When goals conflict with relationships, I seek collaborative resolution.";
  return [speedLabel, riskLabel, peopleLabel];
}

function deriveRiskProfile(riskPreference: string) {
  if (riskPreference === "bold") return "bold / upside-seeking";
  if (riskPreference === "conservative") return "conservative / stability-seeking";
  return "balanced / calibrated";
}

function deriveConflictStyle(peoplePreference: string) {
  if (peoplePreference === "principle") return "principle-first";
  if (peoplePreference === "harmony") return "relationship-first";
  return "collaborative";
}

export interface UserPersonaDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  sessionId: string | null;
}

export function UserPersonaDialog({
  open,
  onOpenChange,
  sessionId,
}: UserPersonaDialogProps) {
  const [saving, setSaving] = useState(false);
  const [loading, setLoading] = useState(false);
  const [initiativeSources, setInitiativeSources] = useState<string[]>([]);
  const [driveScores, setDriveScores] = useState(defaultDriveScores);
  const [values, setValues] = useState<string[]>(Array.from({ length: 5 }, () => ""));
  const [goals, setGoals] = useState({
    currentFocus: "",
    oneYear: "",
    threeYear: "",
  });
  const [traits, setTraits] = useState(defaultTraits);
  const [speedQuality, setSpeedQuality] = useState("balanced");
  const [evidenceStyle, setEvidenceStyle] = useState("balanced");
  const [riskPreference, setRiskPreference] = useState("balanced");
  const [peoplePreference, setPeoplePreference] = useState("collaborative");
  const [nonNegotiables, setNonNegotiables] = useState("");

  const resetFormState = () => {
    setInitiativeSources([]);
    setDriveScores(defaultDriveScores);
    setValues(Array.from({ length: 5 }, () => ""));
    setGoals({
      currentFocus: "",
      oneYear: "",
      threeYear: "",
    });
    setTraits(defaultTraits);
    setSpeedQuality("balanced");
    setEvidenceStyle("balanced");
    setRiskPreference("balanced");
    setPeoplePreference("collaborative");
    setNonNegotiables("");
  };

  const personaProfile = useMemo<UserPersonaProfile>(() => {
    const coreDrives = toDriveEntries(driveScores);
    const sortedDrives = [...coreDrives].sort((a, b) => b.score - a.score);
    const topDrives = sortedDrives.slice(0, 2).map((drive) => drive.label);
    const trimmedValues = values.map((item) => item.trim()).filter(Boolean);
    const decisionStyle = deriveDecisionStyle(speedQuality, evidenceStyle);
    const keyChoices = deriveKeyChoices(speedQuality, riskPreference, peoplePreference);
    const riskProfile = deriveRiskProfile(riskPreference);
    const conflictStyle = deriveConflictStyle(peoplePreference);

    const profile: UserPersonaProfile = {
      version: "persona-v1",
      updated_at: new Date().toISOString(),
      initiative_sources: initiativeSources,
      core_drives: coreDrives,
      top_drives: topDrives,
      values: trimmedValues,
      goals: {
        current_focus: goals.currentFocus.trim(),
        one_year: goals.oneYear.trim(),
        three_year: goals.threeYear.trim(),
      },
      traits: traits,
      decision_style: decisionStyle,
      risk_profile: riskProfile,
      conflict_style: conflictStyle,
      key_choices: keyChoices,
      non_negotiables: nonNegotiables.trim(),
      construction_rules: constructionRules,
      raw_answers: {
        initiativeSources,
        driveScores,
        values,
        goals,
        traits,
        speedQuality,
        evidenceStyle,
        riskPreference,
        peoplePreference,
        nonNegotiables,
      },
      summary: "",
    };
    profile.summary = buildSummary(profile);
    return profile;
  }, [
    driveScores,
    evidenceStyle,
    goals,
    initiativeSources,
    nonNegotiables,
    peoplePreference,
    riskPreference,
    speedQuality,
    traits,
    values,
  ]);

  useEffect(() => {
    if (!open || !sessionId) return;

    setLoading(true);
    resetFormState();
    getSessionPersona(sessionId)
      .then((persona) => {
        if (!persona) {
          return;
        }
        if (persona.initiative_sources) {
          setInitiativeSources(persona.initiative_sources);
        }
        if (persona.core_drives) {
          const nextScores = { ...defaultDriveScores };
          persona.core_drives.forEach((drive) => {
            if (drive.id in nextScores) {
              nextScores[drive.id as DriveKey] = drive.score;
            }
          });
          setDriveScores(nextScores);
        }
        if (persona.values) {
          const nextValues = Array.from({ length: 5 }, (_, idx) => persona.values?.[idx] ?? "");
          setValues(nextValues);
        }
        if (persona.goals) {
          setGoals({
            currentFocus: persona.goals.current_focus ?? "",
            oneYear: persona.goals.one_year ?? "",
            threeYear: persona.goals.three_year ?? "",
          });
        }
        if (persona.traits) {
          const nextTraits = { ...defaultTraits };
          (Object.keys(persona.traits) as TraitKey[]).forEach((key) => {
            if (key in nextTraits) {
              const score = persona.traits?.[key];
              if (typeof score === "number") {
                nextTraits[key] = score;
              }
            }
          });
          setTraits(nextTraits);
        }
        if (persona.decision_style) {
          if (persona.decision_style.includes("speed-first")) {
            setSpeedQuality("speed");
          } else if (persona.decision_style.includes("quality-first")) {
            setSpeedQuality("quality");
          }
          if (persona.decision_style.includes("data-first")) {
            setEvidenceStyle("data");
          } else if (persona.decision_style.includes("intuition")) {
            setEvidenceStyle("intuition");
          }
        }
        if (persona.risk_profile?.includes("bold")) {
          setRiskPreference("bold");
        } else if (persona.risk_profile?.includes("conservative")) {
          setRiskPreference("conservative");
        }
        if (persona.conflict_style?.includes("principle")) {
          setPeoplePreference("principle");
        } else if (persona.conflict_style?.includes("relationship")) {
          setPeoplePreference("harmony");
        }
        if (persona.non_negotiables) {
          setNonNegotiables(persona.non_negotiables);
        }
      })
      .catch((error) => {
        console.warn("[UserPersonaDialog] Failed to load persona", error);
      })
      .finally(() => setLoading(false));
  }, [open, sessionId]);

  const handleToggleInitiative = (value: string) => {
    setInitiativeSources((prev) =>
      prev.includes(value) ? prev.filter((item) => item !== value) : [...prev, value],
    );
  };

  const handleDriveChange = (key: DriveKey, score: number) => {
    setDriveScores((prev) => ({ ...prev, [key]: score }));
  };

  const handleTraitChange = (key: TraitKey, score: number) => {
    setTraits((prev) => ({ ...prev, [key]: score }));
  };

  const handleValueChange = (index: number, value: string) => {
    setValues((prev) => {
      const next = [...prev];
      next[index] = value;
      return next;
    });
  };

  const handleSubmit = async () => {
    if (!sessionId) {
      toast.error("Session not ready", "Please start a session before saving.");
      return;
    }

    setSaving(true);
    try {
      await updateSessionPersona(sessionId, personaProfile);
      toast.success("Persona saved", "Your profile will steer proactive questions.");
      onOpenChange(false);
    } catch (error) {
      toast.error("Unable to save persona", "Check your connection and retry.");
      console.warn("[UserPersonaDialog] Failed to save persona", error);
    } finally {
      setSaving(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] max-w-3xl overflow-y-auto rounded-3xl">
        <DialogHeader className="space-y-2">
          <DialogTitle>主动式人格配置</DialogTitle>
          <DialogDescription>
            我会问一些问题来了解你的驱动力、目标和决策风格，并生成一个核心人格配置。
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-8">
          <section className="space-y-3 rounded-2xl border border-border/60 bg-muted/30 p-4">
            <h3 className="text-sm font-semibold text-foreground">构建规则</h3>
            <ul className="space-y-1 text-xs text-muted-foreground">
              {constructionRules.map((rule) => (
                <li key={rule}>• {rule}</li>
              ))}
            </ul>
          </section>

          <section className="space-y-4">
            <h3 className="text-sm font-semibold text-foreground">主动性来源</h3>
            <p className="text-xs text-muted-foreground">
              请选择你主动行动最常见的触发来源（可多选）。
            </p>
            <div className="flex flex-wrap gap-2">
              {initiativeOptions.map((option) => {
                const active = initiativeSources.includes(option);
                return (
                  <button
                    key={option}
                    type="button"
                    className={cn(
                      "rounded-full border px-3 py-1 text-xs font-semibold transition",
                      active
                        ? "border-primary/60 bg-primary/10 text-primary"
                        : "border-border/50 bg-background text-muted-foreground",
                    )}
                    onClick={() => handleToggleInitiative(option)}
                  >
                    {option}
                  </button>
                );
              })}
            </div>
          </section>

          <section className="space-y-4">
            <h3 className="text-sm font-semibold text-foreground">核心能动性（心理学动机）</h3>
            <p className="text-xs text-muted-foreground">
              参考自自我决定理论与动机研究，请为每项打分（1=低，5=高）。
            </p>
            <div className="grid gap-3 md:grid-cols-2">
              {driveOptions.map((drive) => (
                <div
                  key={drive.id}
                  className="rounded-2xl border border-border/60 bg-background p-3"
                >
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-semibold text-foreground">
                      {drive.label}
                    </span>
                    <span className="text-xs text-muted-foreground">
                      {driveScores[drive.id]}/5
                    </span>
                  </div>
                  <input
                    type="range"
                    min={1}
                    max={5}
                    value={driveScores[drive.id]}
                    onChange={(event) =>
                      handleDriveChange(drive.id, Number(event.target.value))
                    }
                    className="mt-2 w-full accent-primary"
                  />
                </div>
              ))}
            </div>
          </section>

          <section className="space-y-4">
            <h3 className="text-sm font-semibold text-foreground">目标与价值观</h3>
            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-2">
                <label className="text-xs font-semibold text-foreground">
                  当前最重要的目标
                </label>
                <Textarea
                  value={goals.currentFocus}
                  onChange={(event) =>
                    setGoals((prev) => ({
                      ...prev,
                      currentFocus: event.target.value,
                    }))
                  }
                  placeholder="我最近最关注的是……"
                />
              </div>
              <div className="space-y-2">
                <label className="text-xs font-semibold text-foreground">
                  1 年目标
                </label>
                <Textarea
                  value={goals.oneYear}
                  onChange={(event) =>
                    setGoals((prev) => ({ ...prev, oneYear: event.target.value }))
                  }
                  placeholder="一年内我希望……"
                />
              </div>
              <div className="space-y-2 md:col-span-2">
                <label className="text-xs font-semibold text-foreground">
                  3 年目标
                </label>
                <Textarea
                  value={goals.threeYear}
                  onChange={(event) =>
                    setGoals((prev) => ({
                      ...prev,
                      threeYear: event.target.value,
                    }))
                  }
                  placeholder="三年内我希望……"
                />
              </div>
            </div>
            <div className="space-y-2">
              <label className="text-xs font-semibold text-foreground">
                你的核心价值观（最多 5 个）
              </label>
              <div className="grid gap-2 md:grid-cols-2">
                {values.map((value, index) => (
                  <Input
                    key={`value-${index}`}
                    value={value}
                    onChange={(event) => handleValueChange(index, event.target.value)}
                    placeholder={`价值观 #${index + 1}`}
                  />
                ))}
              </div>
            </div>
          </section>

          <section className="space-y-4">
            <h3 className="text-sm font-semibold text-foreground">性格维度（Big Five）</h3>
            <p className="text-xs text-muted-foreground">
              1=低，7=高。用于刻画你的行为倾向。
            </p>
            <div className="grid gap-3 md:grid-cols-2">
              {traitOptions.map((trait) => (
                <div
                  key={trait.id}
                  className="rounded-2xl border border-border/60 bg-background p-3"
                >
                  <div className="flex items-center justify-between">
                    <span className="text-xs font-semibold text-foreground">
                      {trait.label}
                    </span>
                    <span className="text-xs text-muted-foreground">
                      {traits[trait.id]}/7
                    </span>
                  </div>
                  <input
                    type="range"
                    min={1}
                    max={7}
                    value={traits[trait.id]}
                    onChange={(event) =>
                      handleTraitChange(trait.id, Number(event.target.value))
                    }
                    className="mt-2 w-full accent-primary"
                  />
                </div>
              ))}
            </div>
          </section>

          <section className="space-y-4">
            <h3 className="text-sm font-semibold text-foreground">关键决策偏好</h3>
            <div className="grid gap-4 md:grid-cols-2">
              <label className="flex flex-col gap-2 text-xs font-semibold text-foreground">
                速度 vs 质量
                <select
                  className="rounded-lg border border-border/60 bg-background px-3 py-2 text-sm"
                  value={speedQuality}
                  onChange={(event) => setSpeedQuality(event.target.value)}
                >
                  <option value="speed">更偏速度</option>
                  <option value="balanced">平衡</option>
                  <option value="quality">更偏质量</option>
                </select>
              </label>
              <label className="flex flex-col gap-2 text-xs font-semibold text-foreground">
                证据 vs 直觉
                <select
                  className="rounded-lg border border-border/60 bg-background px-3 py-2 text-sm"
                  value={evidenceStyle}
                  onChange={(event) => setEvidenceStyle(event.target.value)}
                >
                  <option value="data">证据优先</option>
                  <option value="balanced">平衡</option>
                  <option value="intuition">直觉优先</option>
                </select>
              </label>
              <label className="flex flex-col gap-2 text-xs font-semibold text-foreground">
                风险偏好
                <select
                  className="rounded-lg border border-border/60 bg-background px-3 py-2 text-sm"
                  value={riskPreference}
                  onChange={(event) => setRiskPreference(event.target.value)}
                >
                  <option value="bold">大胆型</option>
                  <option value="balanced">平衡型</option>
                  <option value="conservative">稳健型</option>
                </select>
              </label>
              <label className="flex flex-col gap-2 text-xs font-semibold text-foreground">
                原则 vs 关系
                <select
                  className="rounded-lg border border-border/60 bg-background px-3 py-2 text-sm"
                  value={peoplePreference}
                  onChange={(event) => setPeoplePreference(event.target.value)}
                >
                  <option value="principle">原则优先</option>
                  <option value="collaborative">协作平衡</option>
                  <option value="harmony">关系优先</option>
                </select>
              </label>
            </div>
          </section>

          <section className="space-y-4">
            <h3 className="text-sm font-semibold text-foreground">底层不可妥协</h3>
            <Textarea
              value={nonNegotiables}
              onChange={(event) => setNonNegotiables(event.target.value)}
              placeholder="对你来说，绝对不能被挑战的底层原则是什么？"
            />
          </section>

          <section className="space-y-3 rounded-2xl border border-dashed border-border/60 bg-muted/20 p-4">
            <h3 className="text-sm font-semibold text-foreground">人格配置预览</h3>
            <p className="text-xs text-muted-foreground">
              {personaProfile.summary || "填写更多信息后会生成摘要。"}
            </p>
          </section>

          {loading && (
            <div className="rounded-2xl border border-border/60 bg-muted/20 p-3 text-xs text-muted-foreground">
              Loading previous persona data…
            </div>
          )}
        </div>

        <DialogFooter className="mt-6">
          <Button variant="outline" type="button" onClick={() => onOpenChange(false)}>
            取消
          </Button>
          <Button type="button" onClick={handleSubmit} disabled={saving}>
            {saving ? "保存中…" : "保存人格配置"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
