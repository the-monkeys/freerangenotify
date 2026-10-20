const { test } = require('node:test');
const assert = require('node:assert/strict');
const { join } = require('node:path');
const { FreeRangeNotify } = require(join(process.env.FRN_SDK_TEST_OUTPUT, 'index.js'));
for (const wrapped of [false,true]) test(`user CRUD ${wrapped ? 'current envelope' : 'legacy flat'}`, async () => {
 const original=global.fetch;
 const user={user_id:'fixture-user',email:'alice@example.test'};
 global.fetch=async()=>Response.json(wrapped?{success:true,data:user}:user);
 try {
 const client=new FreeRangeNotify('fixture-key',{baseURL:'http://fixture.test/v1'});
 assert.equal((await client.users.create({email:user.email})).user_id,user.user_id);
 assert.equal((await client.users.get(user.user_id)).user_id,user.user_id);
 assert.equal((await client.users.update(user.user_id,{full_name:'Alice'})).user_id,user.user_id);
 } finally {global.fetch=original}
});
