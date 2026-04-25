// FreeRangeNotify JS SDK — webhook builder helpers.
//
// These helpers construct NotificationSendParams pre-populated for the
// rich-content webhook channel. They make sending Discord/Slack/Teams
// notifications a one-liner without requiring callers to remember the
// `channel` + `webhook_target` conventions.
//
// Per-provider rendering capabilities (matrix below) are decided server-side
// by the worker's renderer; the builders simply set the optional rich
// fields. Fields not supported by a target degrade gracefully — for example
// polls render as an interactive native poll on Discord, and as a numbered
// list on Slack/Teams (incoming-webhook platform limitation).
//
//   +-------------+---------+-------+-------+---------+
//   | Field       | Discord | Slack | Teams | Generic |
//   +-------------+---------+-------+-------+---------+
//   | attachments |   yes   |  yes  |  yes  |   raw   |
//   | actions     |   yes   |  yes  |  yes  |   raw   |
//   | fields      |   yes   |  yes  |  yes  |   raw   |
//   | poll        |  native |  list |  list |   raw   |
//   | style.color |   yes   |  bar  |  yes  |   n/a   |
//   | mentions    |   yes   |  yes  |  yes  |   n/a   |
//   +-------------+---------+-------+-------+---------+

import type {
    NotificationSendParams,
    ContentPollChoice,
} from './types';

export type Severity = 'info' | 'success' | 'warning' | 'danger' | 'critical';

/**
 * Construct a NotificationSendParams for the webhook channel routed to the
 * named custom provider on the application.
 *
 * `target` must match the `Name` of a registered custom provider on the
 * application.
 */
export function newWebhookNotification(
    target: string,
    title: string,
    body: string,
): NotificationSendParams {
    return {
        user_id: '',
        channel: 'webhook',
        title,
        body,
        webhook_target: target,
    };
}

/**
 * Pre-configured Discord alert. Severity is mapped server-side to an embed
 * color preset (info/success/warning/danger/critical → blue/green/orange/red).
 *
 * `target` must match the `Name` of a Discord custom provider on the app
 * (kind === 'discord', channel === 'webhook').
 */
export function newDiscordAlert(
    target: string,
    title: string,
    body: string,
    severity?: Severity,
): NotificationSendParams {
    const p = newWebhookNotification(target, title, body);
    if (severity) p.style = { severity: severity === 'critical' ? 'danger' : severity };
    return p;
}

/**
 * Pre-configured Slack alert. Severity drives the colored sidebar bar on the
 * message attachment.
 *
 * `target` must match the `Name` of a Slack custom provider on the app
 * (kind === 'slack', channel === 'webhook').
 */
export function newSlackAlert(
    target: string,
    title: string,
    body: string,
    severity?: Severity,
): NotificationSendParams {
    const p = newWebhookNotification(target, title, body);
    if (severity) p.style = { severity: severity === 'critical' ? 'danger' : severity };
    return p;
}

/**
 * Pre-configured Microsoft Teams alert. Severity drives the connector card
 * themeColor.
 *
 * `target` must match the `Name` of a Teams custom provider on the app
 * (kind === 'teams', channel === 'webhook').
 */
export function newTeamsAlert(
    target: string,
    title: string,
    body: string,
    severity?: Severity,
): NotificationSendParams {
    const p = newWebhookNotification(target, title, body);
    if (severity) p.style = { severity: severity === 'critical' ? 'danger' : severity };
    return p;
}

/**
 * Convenience helper to attach a poll. Discord renders this as a native
 * interactive poll. Slack and Teams have no native poll element on
 * incoming webhooks and fall back to a numbered list of choices.
 */
export function withPoll(
    params: NotificationSendParams,
    question: string,
    choices: string[],
): NotificationSendParams {
    return {
        ...params,
        poll: {
            question,
            choices: choices.map<ContentPollChoice>((label) => ({ label })),
        },
    };
}

/** Namespaced re-export so users can write `webhook.discord(...)` etc. */
export const webhook = {
    notification: newWebhookNotification,
    discord: newDiscordAlert,
    slack: newSlackAlert,
    teams: newTeamsAlert,
    withPoll,
};
