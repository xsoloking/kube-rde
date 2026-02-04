import { useEffect, useRef } from 'react';

export function useAdaptivePolling(callback: () => void, getInterval: () => number) {
  const savedCallback = useRef(callback);
  const savedGetInterval = useRef(getInterval);

  useEffect(() => {
    savedCallback.current = callback;
  }, [callback]);

  useEffect(() => {
    savedGetInterval.current = getInterval;
  }, [getInterval]);

  useEffect(() => {
    let timeoutId: ReturnType<typeof setTimeout>;

    const tick = () => {
      if (document.hidden) return;
      savedCallback.current();
      schedule();
    };

    const schedule = () => {
      const interval = savedGetInterval.current();
      timeoutId = setTimeout(tick, interval);
    };

    const handleVisibilityChange = () => {
      clearTimeout(timeoutId);
      if (!document.hidden) {
        // Fetch immediately when tab becomes visible, then resume scheduling
        savedCallback.current();
        schedule();
      }
    };

    schedule();
    document.addEventListener('visibilitychange', handleVisibilityChange);

    return () => {
      clearTimeout(timeoutId);
      document.removeEventListener('visibilitychange', handleVisibilityChange);
    };
  }, []);
}
