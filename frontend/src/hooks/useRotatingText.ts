import { useEffect, useState } from "react";
export function useRotatingText(texts: string[], intervalMs: number = 3000) {
    const [index, setIndex] = useState(0);
    useEffect(() => {
        if (texts.length < 2) {
            return;
        }
        const intervalId = window.setInterval(() => setIndex((prev) => (prev + 1) % texts.length), intervalMs);
        return () => window.clearInterval(intervalId);
    }, [texts, intervalMs]);
    return texts[index % texts.length] ?? "";
}
