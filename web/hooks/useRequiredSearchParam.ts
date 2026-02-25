"use client";

import { useMemo } from "react";
import { useSearchParams } from "next/navigation";

export type UseRequiredSearchParamResult = {
  value: string;
  missing: boolean;
};

export type UseRequiredSearchParamOptions = {
  trim?: boolean;
};

export function useRequiredSearchParam(
  key: string,
  options?: UseRequiredSearchParamOptions,
): UseRequiredSearchParamResult {
  const searchParams = useSearchParams();
  const shouldTrim = options?.trim ?? false;

  return useMemo(() => {
    const rawValue = searchParams.get(key);
    if (rawValue === null) {
      return { value: "", missing: true };
    }

    const value = shouldTrim ? rawValue.trim() : rawValue;
    if (value.length === 0) {
      return { value: "", missing: true };
    }

    return { value, missing: false };
  }, [key, searchParams, shouldTrim]);
}
