import { FreeRangeNotify } from '@freerangenotify/sdk';

// Backend only. Supply your saved app ID and secret from server configuration.
export async function sendWelcome(apiKey: string, appId: string, baseURL: string) {
 const client = new FreeRangeNotify(apiKey, { baseURL });
 const user = await client.users.create({ email:'alice@example.test', external_id:'alice' });
 const template = await client.templates.create({ app_id:appId, name:'welcome_email', channel:'email', subject:'Welcome', body:'Hello {{.name}}', variables:['name'], locale:'en' });
 return client.notifications.send({ user_id:user.user_id, template_id:template.id, channel:'email', data:{name:'Alice'} });
}

// Browser: token comes from an authenticated backend. No application key here.
export function listen(baseURL: string, sseToken: string) {
 const url = new URL(baseURL.replace(/\/+$/, '') + '/sse');
 url.searchParams.set('sse_token', sseToken);
 const stream = new EventSource(url);
 stream.addEventListener('notification', event => console.log(JSON.parse(event.data)));
 return () => stream.close();
}
