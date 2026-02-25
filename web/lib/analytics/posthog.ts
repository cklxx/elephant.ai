'use client';

import posthog, { type Properties } from 'posthog-js';

type PendingEvent = {
  name: string;
  properties?: Properties;
};

const pendingQueue: PendingEvent[] = [];
const MAX_PENDING_EVENTS = 50;

let hasInitialized = false;
let initAttempted = false;

const defaultHost = 'https://app.posthog.com';

function flushQueue() {
  if (!hasInitialized) {
    return;
  }

  while (pendingQueue.length > 0) {
    const event = pendingQueue.shift();
    if (!event) {
      continue;
    }
    posthog.capture(event.name, event.properties);
  }
}

function enqueueEvent(event: PendingEvent) {
  if (pendingQueue.length >= MAX_PENDING_EVENTS) {
    pendingQueue.shift();
  }
  pendingQueue.push(event);
}

export function initAnalytics() {
  if (typeof window === 'undefined') {
    return;
  }

  if (hasInitialized || initAttempted) {
    return;
  }

  initAttempted = true;

  const apiKey = process.env.NEXT_PUBLIC_POSTHOG_KEY;
  if (!apiKey) {
    if (process.env.NODE_ENV !== 'production' && process.env.NODE_ENV !== 'test') {
      console.info('[analytics] PostHog disabled: NEXT_PUBLIC_POSTHOG_KEY not set');
    }
    return;
  }

  const apiHost = process.env.NEXT_PUBLIC_POSTHOG_HOST || defaultHost;

  try {
    posthog.init(apiKey, {
      api_host: apiHost,
      autocapture: true,
      capture_pageview: true,
      capture_pageleave: true,
      disable_session_recording: true,
      persistence: 'localStorage+cookie',
      loaded: () => {
        hasInitialized = true;
        flushQueue();
      },
    });

    hasInitialized = true;
    flushQueue();
  } catch (error) {
    if (process.env.NODE_ENV !== 'production' && process.env.NODE_ENV !== 'test') {
      console.warn('[analytics] Failed to initialize PostHog', error);
    }
  }
}

export function captureEvent(name: string, properties: Properties = {}) {
  if (typeof window === 'undefined') {
    return;
  }

  const payload: Properties = { ...properties };
  if (!('source' in payload)) {
    payload.source = 'web_app';
  }

  if (!hasInitialized) {
    enqueueEvent({ name, properties: payload });
    return;
  }

  posthog.capture(name, payload);
}

export function resetAnalytics() {
  if (!hasInitialized) {
    return;
  }

  posthog.reset();
}
