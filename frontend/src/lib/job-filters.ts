export function dollarsFromBudgetQueryParam(
  centsParam: string | null,
  dollarsParam: string | null
): string {
  if (centsParam != null && centsParam !== "") {
    const n = Number(centsParam);
    if (!Number.isFinite(n)) {
      return "";
    }
    return String(n / 100);
  }
  return dollarsParam || "";
}

export function centsFromDollarInput(raw: string): number | undefined {
  const t = raw.trim();
  if (!t) {
    return undefined;
  }
  const n = Number(t);
  if (!Number.isFinite(n)) {
    return undefined;
  }
  return Math.round(n * 100);
}

export function createDebounced<T extends (...args: never[]) => void>(
  fn: T,
  waitMs: number
): T & { cancel: () => void } {
  let timer: ReturnType<typeof setTimeout> | undefined;
  const wrapped = ((...args: Parameters<T>) => {
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => {
      fn(...args);
    }, waitMs);
  }) as T & { cancel: () => void };
  wrapped.cancel = () => {
    if (timer) clearTimeout(timer);
  };
  return wrapped;
}

/** When parent search becomes empty while the input still has a draft, cancel the pending flush. */
export function syncSearchDraftFromParent(
  parentSearch: string,
  currentDraft: string
): { cancelPending: boolean; draft: string } {
  if (parentSearch === "" && currentDraft !== "") {
    return { cancelPending: true, draft: "" };
  }
  return { cancelPending: false, draft: parentSearch };
}
