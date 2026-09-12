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
