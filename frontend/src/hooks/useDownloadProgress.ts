import { useSyncExternalStore } from "react";
import { GetDownloadProgress } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
export interface DownloadProgressInfo {
    is_downloading: boolean;
    mb_downloaded: number;
    speed_mbps: number;
    rate_limited?: boolean;
    rate_limit_secs?: number;
    cooldown?: boolean;
    cooldown_secs?: number;
    cooldown_message?: string;
    cooldown_event_id?: number;
}
let snapshot: DownloadProgressInfo = {
    is_downloading: false,
    mb_downloaded: 0,
    speed_mbps: 0,
    rate_limited: false,
    rate_limit_secs: 0,
    cooldown: false,
    cooldown_secs: 0,
    cooldown_message: "",
};
const listeners = new Set<() => void>();
let listening = false;
function publish(progress: DownloadProgressInfo) {
    snapshot = progress;
    for (const listener of listeners) {
        listener();
    }
}
function startListening() {
    listening = true;
    let receivedEvent = false;
    EventsOn("download-progress", (progress: DownloadProgressInfo) => {
        receivedEvent = true;
        publish(progress);
    });
    GetDownloadProgress()
        .then((progress) => {
            if (!receivedEvent) {
                publish(progress);
            }
        })
        .catch((error) => console.error("Failed to get download progress:", error));
}
function subscribe(listener: () => void) {
    if (!listening) {
        startListening();
    }
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
}
function getSnapshot() {
    return snapshot;
}
export function useDownloadProgress() {
    return useSyncExternalStore(subscribe, getSnapshot);
}
