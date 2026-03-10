/**
 * Timezone utilities using native Intl API (no external deps).
 */

/**
 * Get the UTC offset in minutes for a given timezone at a specific date.
 * Positive = timezone is ahead of UTC (e.g. +330 for IST).
 */
function getTimezoneOffsetMs(date: Date, timeZone: string): number {
  const formatter = new Intl.DateTimeFormat('en-US', {
    timeZone,
    timeZoneName: 'longOffset',
  });
  const parts = formatter.formatToParts(date);
  const tzPart = parts.find((p) => p.type === 'timeZoneName')?.value || '';
  // Parse "GMT-4", "GMT+5:30", "GMT+05:30", "GMT-04:00"
  const match = tzPart.match(/GMT([+-])(\d{1,2}):?(\d{2})?/);
  if (!match) return 0;
  const sign = match[1] === '+' ? 1 : -1;
  const hours = parseInt(match[2], 10);
  const minutes = match[3] ? parseInt(match[3], 10) : 0;
  const totalMinutes = sign * (hours * 60 + minutes);
  // For "GMT-5", totalMinutes = -300. Local is 5h behind UTC, so UTC = local + 5h.
  return -totalMinutes * 60 * 1000; // ms to add to local (as if UTC) to get real UTC
}

/**
 * Convert a datetime string (YYYY-MM-DDTHH:mm or with seconds) representing LOCAL time
 * in the given timezone to an ISO UTC string.
 */
export function localInTimezoneToISO(datetimeLocal: string, timeZone: string): string | undefined {
  if (!datetimeLocal || !timeZone) return undefined;
  const withSeconds = datetimeLocal.length === 16 ? `${datetimeLocal}:00` : datetimeLocal;
  const normalized = withSeconds.replace('T', ' ').trim();
  const [datePart, timePart] = normalized.split(' ');
  if (!datePart || !timePart) return undefined;
  const [y, m, d] = datePart.split('-').map(Number);
  const [hh, mm, ss = 0] = timePart.split(':').map(Number);
  if (isNaN(y) || isNaN(m) || isNaN(d) || isNaN(hh) || isNaN(mm)) return undefined;

  try {
    // Use noon UTC on that day to get the offset (avoids DST edge at midnight)
    const probe = new Date(Date.UTC(y, m - 1, d, 12, 0, 0));
    const offsetMs = getTimezoneOffsetMs(probe, timeZone);
    // Local time in TZ is offset behind UTC, so UTC = local + offset
    const localAsUtc = Date.UTC(y, m - 1, d, hh, mm, ss || 0);
    const utcMs = localAsUtc + offsetMs;
    return new Date(utcMs).toISOString();
  } catch {
    return new Date(datetimeLocal).toISOString(); // fallback to browser local
  }
}

/**
 * Format a UTC date (ISO string or Date) in the given timezone as "yyyy-MM-dd'T'HH:mm".
 */
export function formatInTimezone(utcDate: Date | string, timeZone: string): string {
  const d = typeof utcDate === 'string' ? new Date(utcDate) : utcDate;
  if (isNaN(d.getTime())) return '';
  try {
    const formatter = new Intl.DateTimeFormat('en-CA', {
      timeZone,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: undefined,
      hour12: false,
    });
    const parts = formatter.formatToParts(d);
    const get = (type: string) => parts.find((p) => p.type === type)?.value ?? '';
    const year = get('year');
    const month = get('month');
    const dayNum = get('day');
    const hour = get('hour');
    const minute = get('minute');
    return `${year}-${month}-${dayNum}T${hour.padStart(2, '0')}:${minute.padStart(2, '0')}`;
  } catch {
    return d.toISOString().slice(0, 16);
  }
}
