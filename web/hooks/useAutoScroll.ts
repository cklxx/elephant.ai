/**
 * Auto-Scroll Hook
 *
 * Provides automatic scrolling behavior for scrollable containers with:
 * - Auto-scroll to bottom when content updates
 * - Manual scroll lock when user scrolls up
 * - Configurable threshold for scroll lock
 * - Smooth scrolling behavior
 *
 * @example
 * ```tsx
 * const containerRef = useAutoScroll([events.length], {
 *   enabled: true,
 *   threshold: 100,
 *   behavior: 'smooth'
 * });
 *
 * return <div ref={containerRef}>{events.map(...)}</div>;
 * ```
 */

import { useCallback, useEffect, useRef, DependencyList } from 'react';

/**
 * Configuration options for auto-scroll behavior
 */
interface UseAutoScrollOptions {
  /** Whether auto-scroll is enabled (default: true) */
  enabled?: boolean;
  /** Distance from bottom to consider "at bottom" in pixels (default: 100) */
  threshold?: number;
  /** Scroll behavior type (default: 'smooth') */
  behavior?: ScrollBehavior;
  /** Delay before scrolling in milliseconds (default: 100) */
  delay?: number;
}

/**
 * Hook for automatic scrolling to bottom
 *
 * Automatically scrolls to the bottom of a container when dependencies change,
 * but respects user scroll position and only auto-scrolls if user is near the bottom.
 *
 * @param dependencies - Array of dependencies to trigger auto-scroll (e.g., [events.length])
 * @param options - Configuration options for scroll behavior
 * @returns Ref to attach to the scrollable container
 */
export function useAutoScroll<T extends HTMLElement = HTMLDivElement>(
  dependencies: DependencyList,
  options: UseAutoScrollOptions = {}
): React.RefObject<T | null> {
  const {
    enabled = true,
    threshold = 100,
    behavior = 'smooth',
    delay = 100,
  } = options;

  const containerRef = useRef<T>(null);
  const isUserScrollingRef = useRef(false);
  const scrollTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const previousDependenciesRef = useRef<DependencyList | null>(null);
  const previousEnabledRef = useRef(enabled);

  /**
   * Check if container is scrolled near the bottom
   */
  const isNearBottom = useCallback(
    (element: HTMLElement): boolean => {
      const { scrollTop, scrollHeight, clientHeight } = element;
      return scrollHeight - scrollTop - clientHeight < threshold;
    },
    [threshold],
  );

  /**
   * Scroll container to bottom
   */
  const scrollToBottom = useCallback(
    (element: HTMLElement) => {
      element.scrollTo({
        top: element.scrollHeight,
        behavior,
      });
    },
    [behavior],
  );

  /**
   * Handle user scroll events
   * Detect if user manually scrolled away from bottom
   */
  useEffect(() => {
    const container = containerRef.current;
    if (!container || !enabled) return;

    const handleScroll = () => {
      // Clear any pending scroll timeout
      if (scrollTimeoutRef.current) {
        clearTimeout(scrollTimeoutRef.current);
      }

      // Check if user scrolled up
      isUserScrollingRef.current = !isNearBottom(container);

      // Reset user scrolling flag after delay
      scrollTimeoutRef.current = setTimeout(() => {
        if (isNearBottom(container)) {
          isUserScrollingRef.current = false;
        }
      }, delay);
    };

    container.addEventListener('scroll', handleScroll, { passive: true });

    return () => {
      container.removeEventListener('scroll', handleScroll);
      if (scrollTimeoutRef.current) {
        clearTimeout(scrollTimeoutRef.current);
      }
    };
  }, [enabled, delay, isNearBottom]);

  /**
   * Auto-scroll when dependencies change
   * Only scrolls if user hasn't manually scrolled away from bottom
   */
  useEffect(() => {
    const container = containerRef.current;
    const previousEnabled = previousEnabledRef.current;
    previousEnabledRef.current = enabled;

    if (!container || !enabled) {
      previousDependenciesRef.current = dependencies;
      return;
    }

    const previousDependencies = previousDependenciesRef.current;
    previousDependenciesRef.current = dependencies;

    const dependenciesUnchanged =
      previousDependencies &&
      previousDependencies.length === dependencies.length &&
      dependencies.every((dep, idx) => Object.is(dep, previousDependencies[idx]));

    if (dependenciesUnchanged && previousEnabled === enabled) {
      return;
    }

    // Don't auto-scroll if user is manually scrolling
    if (isUserScrollingRef.current) return;

    // Only auto-scroll if already near bottom or first content
    if (container.scrollHeight <= container.clientHeight || isNearBottom(container)) {
      // Small delay to ensure DOM has updated
      const timeoutId = setTimeout(() => {
        scrollToBottom(container);
      }, delay);

      return () => clearTimeout(timeoutId);
    }
  });

  return containerRef;
}

/**
 * Simplified version that always scrolls to bottom
 *
 * @example
 * ```tsx
 * const containerRef = useScrollToBottom([events.length]);
 * return <div ref={containerRef}>{events.map(...)}</div>;
 * ```
 */
export function useScrollToBottom<T extends HTMLElement = HTMLDivElement>(
  dependencies: DependencyList,
  behavior: ScrollBehavior = 'smooth'
): React.RefObject<T | null> {
  const containerRef = useRef<T>(null);
  const previousDependenciesRef = useRef<DependencyList | null>(null);
  const previousBehaviorRef = useRef(behavior);

  useEffect(() => {
    const container = containerRef.current;
    const previousBehavior = previousBehaviorRef.current;
    previousBehaviorRef.current = behavior;

    if (!container) {
      previousDependenciesRef.current = dependencies;
      return;
    }

    const previousDependencies = previousDependenciesRef.current;
    previousDependenciesRef.current = dependencies;

    const dependenciesUnchanged =
      previousDependencies &&
      previousDependencies.length === dependencies.length &&
      dependencies.every((dep, idx) => Object.is(dep, previousDependencies[idx]));

    if (dependenciesUnchanged && Object.is(previousBehavior, behavior)) {
      return;
    }

    const timeoutId = setTimeout(() => {
      container.scrollTo({
        top: container.scrollHeight,
        behavior,
      });
    }, 100);

    return () => clearTimeout(timeoutId);
  });

  return containerRef;
}
