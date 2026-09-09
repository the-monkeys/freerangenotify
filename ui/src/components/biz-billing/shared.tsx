import type { ReactNode } from 'react';
import { Badge } from '../ui/badge';

export type BizPanelProps = { apiKey: string };

export const paisa = (n = 0) => (n / 100).toFixed(2);

export function statusBadge(status: string): ReactNode {
  const variants: Record<string, 'default' | 'secondary' | 'destructive' | 'outline'> = {
    active: 'default',
    trialing: 'default',
    paid: 'default',
    success: 'default',
    sent: 'default',
    accepted: 'default',
    open: 'default',
    draft: 'secondary',
    paused: 'secondary',
    pending: 'secondary',
    expired: 'secondary',
    canceled: 'destructive',
    terminated: 'destructive',
    rejected: 'destructive',
    void: 'destructive',
    failed: 'destructive',
    past_due: 'destructive',
    overdue: 'destructive',
    refunded: 'outline',
    converted: 'outline',
  };
  return <Badge variant={variants[status] || 'secondary'}>{status}</Badge>;
}
