import { useEffect, useRef, useState } from "react";

const DEFAULT_SKELETON_MIN_DURATION_MS = (() => {
  const raw = Number(import.meta.env.VITE_UI_SKELETON_MIN_MS ?? "");
  if (!Number.isFinite(raw) || raw < 0) {
    return 300;
  }
  return Math.round(raw);
})();

export function useMinimumLoading(loading: boolean, minDurationMS = DEFAULT_SKELETON_MIN_DURATION_MS): boolean {
  const [visible, setVisible] = useState(Boolean(loading));
  const loadingStartedAtRef = useRef<number>(loading ? Date.now() : 0);
  const hideTimeoutRef = useRef<number | null>(null);

  useEffect(() => {
    if (loading) {
      if (hideTimeoutRef.current !== null) {
        window.clearTimeout(hideTimeoutRef.current);
        hideTimeoutRef.current = null;
      }
      if (!loadingStartedAtRef.current) {
        loadingStartedAtRef.current = Date.now();
      }
      setVisible(true);
      return;
    }

    if (!visible) {
      loadingStartedAtRef.current = 0;
      return;
    }

    const startedAt = loadingStartedAtRef.current || Date.now();
    const elapsed = Date.now() - startedAt;
    const waitFor = Math.max(0, minDurationMS - elapsed);

    if (waitFor === 0) {
      loadingStartedAtRef.current = 0;
      setVisible(false);
      return;
    }

    hideTimeoutRef.current = window.setTimeout(() => {
      hideTimeoutRef.current = null;
      loadingStartedAtRef.current = 0;
      setVisible(false);
    }, waitFor);

    return () => {
      if (hideTimeoutRef.current !== null) {
        window.clearTimeout(hideTimeoutRef.current);
        hideTimeoutRef.current = null;
      }
    };
  }, [loading, minDurationMS, visible]);

  return visible;
}
