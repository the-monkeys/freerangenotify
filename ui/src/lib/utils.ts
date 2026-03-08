import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function timeAgo(dateStr: string): string {
  const seconds = Math.floor((Date.now() - new Date(dateStr).getTime()) / 1000);
  if (seconds < 60) return 'just now';
  const minutes = Math.floor(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  return `${months}mo ago`;
}

export function formatDuration(startStr?: string, endStr?: string): string {
  if (!startStr || !endStr) return '';
  const ms = new Date(endStr).getTime() - new Date(startStr).getTime();
  if (ms < 1000) return `${ms}ms`;
  if (ms < 60000) return `${(ms / 1000).toFixed(1)}s`;
  return `${(ms / 60000).toFixed(1)}m`;
}

/** Extract a human-readable error message from an Axios error. */
export function extractErrorMessage(err: any, fallback = 'An unexpected error occurred'): string {
  const raw = err?.response?.data?.error;
  return (typeof raw === 'string' ? raw : raw?.message) ||
    err?.response?.data?.message ||
    err?.message ||
    fallback;
}
