import Papa from 'papaparse';
import type { CreateUserRequest } from '../types';

export interface SubscriberCSVRow { row: number; user: CreateUserRequest }
export const SUBSCRIBER_CSV_MAX_BYTES = 10 * 1024 * 1024;
const MAX_ROWS = 10_000;
const MAX_CHUNK_BYTES = 1024 * 1024;

export function parseSubscriberCSV(text: string, timezone: string): SubscriberCSVRow[][] {
    const parsed = Papa.parse<string[]>(text, { delimiter: ',', skipEmptyLines: 'greedy' });
    if (parsed.errors.length) {
        const error = parsed.errors[0];
        throw new Error(`Row ${(error.row ?? 0) + 1}: ${error.message}`);
    }
    const [rawHeaders, ...records] = parsed.data;
    if (!rawHeaders || !records.length) throw new Error('CSV must contain a header and at least one data row.');
    if (records.length > MAX_ROWS) throw new Error(`CSV supports up to ${MAX_ROWS.toLocaleString()} rows per file.`);
    const headers = rawHeaders.map(h => h.trim().toLowerCase());
    if (!headers.includes('email') || new Set(headers).size !== headers.length) {
        throw new Error("CSV needs an 'email' header and unique column names.");
    }
    const chunks: SubscriberCSVRow[][] = [];
    let chunk: SubscriberCSVRow[] = [];
    let bytes = 64;
    for (const [index, columns] of records.entries()) {
        const row = index + 2;
        if (columns.length !== headers.length) throw new Error(`Row ${row}: column count does not match the header.`);
        const field = (name: string) => columns[headers.indexOf(name)]?.trim() || undefined;
        const email = field('email');
        if (!email || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) throw new Error(`Row ${row}: a valid email is required.`);
        const user: CreateUserRequest = {
            email, full_name: field(headers.includes('full_name') ? 'full_name' : 'name'),
            phone: field('phone'), external_id: field('external_id'), timezone, language: 'en',
        };
        const size = new TextEncoder().encode(JSON.stringify(user)).length + 1;
        if (size + 64 > MAX_CHUNK_BYTES) throw new Error(`Row ${row}: subscriber data is too large.`);
        if (chunk.length === 500 || bytes + size > MAX_CHUNK_BYTES) {
            chunks.push(chunk); chunk = []; bytes = 64;
        }
        chunk.push({ row, user }); bytes += size;
    }
    if (chunk.length) chunks.push(chunk);
    return chunks;
}
