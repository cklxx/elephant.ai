"use client";

import { useCallback, useEffect, useState } from "react";

import { getOnboardingState } from "@/lib/api";

export function useOnboardingState() {
  const [onboardingOpen, setOnboardingOpen] = useState(false);
  const [onboardingLoading, setOnboardingLoading] = useState(true);

  const refreshOnboarding = useCallback(async () => {
    setOnboardingLoading(true);
    try {
      const state = await getOnboardingState();
      setOnboardingOpen(!state.completed);
    } catch (error) {
      setOnboardingOpen(false);
    } finally {
      setOnboardingLoading(false);
    }
  }, []);

  useEffect(() => {
    void refreshOnboarding();
  }, [refreshOnboarding]);

  const markOnboardingCompleted = useCallback(() => {
    setOnboardingOpen(false);
    setOnboardingLoading(false);
  }, []);

  return {
    onboardingOpen,
    setOnboardingOpen,
    onboardingLoading,
    refreshOnboarding,
    markOnboardingCompleted,
  };
}

