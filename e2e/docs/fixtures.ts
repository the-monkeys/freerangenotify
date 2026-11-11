import { test as base, expect } from '@playwright/test';
export const test=base.extend<{isolation:void}>({isolation:[async({page},use)=>{
 const unexpected:string[]=[];
 await page.route('**/*',async route=>{const u=new URL(route.request().url());if(u.hostname==='fonts.googleapis.com')return route.fulfill({contentType:'text/css',body:''});if(u.origin==='http://127.0.0.1:3011' && !u.pathname.startsWith('/v1/'))return route.continue();if((u.origin==='http://127.0.0.1:3011' || (process.env.FRN_DOCS_API_BASE && u.origin===new URL(process.env.FRN_DOCS_API_BASE).origin)) && u.pathname.includes('/v1/'))return route.fulfill({json:{data:[],success:true}});unexpected.push(u.href);await route.abort();});
 await use();expect(unexpected,'unexpected external requests').toEqual([]);
 },{auto:true}]});export {expect};
export const spec={
  "swagger": "2.0",
  "info": {
    "title": "Fixture API"
  },
  "paths": {
    "/v1/notifications": {
      "post": {
        "summary": "Send safely",
        "security": [
          {
            "ApiKeyAuth": []
          }
        ],
        "parameters": [
          {
            "name": "body",
            "in": "body",
            "schema": {
              "$ref": "#/definitions/Send"
            }
          }
        ],
        "responses": {
          "202": {
            "description": "Queued",
            "schema": {
              "$ref": "#/definitions/Send"
            }
          }
        }
      }
    }
  },
  "definitions": {
    "Send": {
      "type": "object",
      "required": [
        "channel"
      ],
      "properties": {
        "channel": {
          "type": "string",
          "example": "email"
        },
        "nested": {
          "$ref": "#/definitions/Send"
        }
      }
    }
  }
};
