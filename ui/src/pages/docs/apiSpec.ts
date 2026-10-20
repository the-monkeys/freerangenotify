export interface SwaggerSchema {
    $ref?: string; type?: string; format?: string; description?: string;
    properties?: Record<string, SwaggerSchema>; required?: string[];
    items?: SwaggerSchema; allOf?: SwaggerSchema[]; enum?: unknown[]; example?: unknown;
    additionalProperties?: boolean | SwaggerSchema;
}
export interface SwaggerParameter extends SwaggerSchema { name: string; in: string; required?: never; schema?: SwaggerSchema }
export interface SwaggerOperation {
    summary?: string; description?: string; tags?: string[];
    parameters?: Array<Omit<SwaggerParameter, 'required'> & { required?: boolean }>;
    responses?: Record<string, { description?: string; schema?: SwaggerSchema }>;
    security?: Record<string, string[]>[];
    'x-feature-prerequisites'?: string;
    'x-contract-example'?: { request?: unknown; response?: unknown };
}
export interface SwaggerSpec {
    swagger: '2.0'; info?: {title?: string; description?: string; version?: string};
    basePath?: string; paths: Record<string, Record<string, SwaggerOperation>>;
    definitions?: Record<string, SwaggerSchema>; securityDefinitions?: Record<string, unknown>;
}
export function validateSwagger2(value: unknown): SwaggerSpec {
    if (!value || typeof value !== 'object' || (value as SwaggerSpec).swagger !== '2.0' ||
        !(value as SwaggerSpec).paths || typeof (value as SwaggerSpec).paths !== 'object' || Array.isArray((value as SwaggerSpec).paths)) throw new Error('Invalid Swagger 2 specification');
    for (const methods of Object.values((value as SwaggerSpec).paths)) {
        if (!methods || typeof methods !== 'object' || Array.isArray(methods)) throw new Error('Invalid path operations');
        for (const [method, operation] of Object.entries(methods)) if (['get','post','put','patch','delete','head','options'].includes(method) && (!operation || typeof operation !== 'object' || Array.isArray(operation))) throw new Error('Invalid operation');
    }
    return value as SwaggerSpec;
}
export async function loadApiSpec(url: string, fallbackURL: string, signal?: AbortSignal): Promise<SwaggerSpec> {
    async function load(target: string) { const response=await fetch(target,{signal}); if(!response.ok)throw new Error('Specification unavailable'); return validateSwagger2(await response.json()); }
    try { return await load(url); } catch(error) { if(signal?.aborted)throw error;return load(fallbackURL); }
}
export function resolveSchema(spec: SwaggerSpec, ref: string): SwaggerSchema | undefined {
    const prefix='#/definitions/';if(!ref.startsWith(prefix))return undefined;
    const key=ref.slice(prefix.length).replace(/~1/g,'/').replace(/~0/g,'~');
    return spec.definitions && Object.prototype.hasOwnProperty.call(spec.definitions,key) ? spec.definitions[key] : undefined;
}
export function schemaExample(spec: SwaggerSpec, schema: SwaggerSchema, depth=0, visited=new Set<string>()): unknown {
    if(depth>=4)return null;
    if(schema.$ref){if(visited.has(schema.$ref))return null;const value=resolveSchema(spec,schema.$ref);return value?schemaExample(spec,value,depth+1,new Set([...visited,schema.$ref])):null;}
    if(schema.example!==undefined)return schema.example;if(schema.enum?.length)return schema.enum[0];
    if(schema.allOf)return Object.assign({},...schema.allOf.map(s=>schemaExample(spec,s,depth+1,visited)).filter(v=>v && typeof v==='object'));
    if(schema.type==='array')return schema.items?[schemaExample(spec,schema.items,depth+1,visited)]:[];
    if(schema.properties || schema.type==='object')return Object.fromEntries(Object.entries(schema.properties||{}).slice(0,50).map(([k,v])=>[k,schemaExample(spec,v,depth+1,visited)]));
    if(schema.type==='boolean')return false;if(schema.type==='integer'||schema.type==='number')return 0;
    return schema.format==='date-time'?'2026-01-01T00:00:00Z':schema.type==='file'?'multipart binary file':'string';
}
