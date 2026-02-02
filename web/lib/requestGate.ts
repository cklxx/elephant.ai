export type RequestGate = {
  next: () => number;
  isLatest: (token: number) => boolean;
  reset: () => void;
};

export function createRequestGate(): RequestGate {
  let current = 0;
  return {
    next() {
      current += 1;
      return current;
    },
    isLatest(token) {
      return token === current;
    },
    reset() {
      current += 1;
    },
  };
}
