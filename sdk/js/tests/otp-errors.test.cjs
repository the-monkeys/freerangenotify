const { test } = require('node:test');
const assert = require('node:assert/strict');
const { join } = require('node:path');
const { HttpClient } = require(join(process.env.FRN_SDK_TEST_OUTPUT, 'client.js'));
const { OTPClient, OTPError } = require(join(process.env.FRN_SDK_TEST_OUTPUT, 'otp.js'));
const { FreeRangeNotifyError } = require(join(process.env.FRN_SDK_TEST_OUTPUT, 'errors.js'));

for (const [name, status, body, expected] of [
 ['invalid code',400,'{"error":"invalid_code","attempts_remaining":2}','invalid_code'],
 ['pending',409,'{"error":"operation_pending"}','operation_pending'],
 ['nested middleware',401,'{"error":{"code":"UNAUTHORIZED","message":"token expired"}}',null],
 ['unknown error',400,'{"error":"custom_error"}',null],
 ['invalid JSON',404,'not json',null],
]) test(name, async () => {
 const original = global.fetch;
 global.fetch = async () => new Response(body,{status});
 try {
  const client = new OTPClient(new HttpClient('test-key','http://fixture.test/v1'));
  await assert.rejects(client.verify({ requestId:'otp-fixture',code:'123456' }), error => {
   assert.ok(error instanceof FreeRangeNotifyError);
   if(expected) {assert.ok(error instanceof OTPError);assert.equal(error.code,expected)}
   else assert.equal(error instanceof OTPError,false);
   return true;
  });
 } finally {global.fetch=original}
});
