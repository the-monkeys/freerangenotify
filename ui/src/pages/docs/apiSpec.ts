export interface SwaggerSchema {
    $ref?: string; type?: string; format?: string; description?: string;
    properties?: Record<string, SwaggerSchema>; required?: string[];
    items?: SwaggerSchema; allOf?: SwaggerSchema[]; enum?: unknown[]; example?: unknown;
    additionalProperties?: boolean | SwaggerSchema;
}
export type SwaggerParameter = Omit<SwaggerSchema, 'required'> & {name:string; in:string; required?:boolean; schema?:SwaggerSchema};
export interface SwaggerOperation {
    summary?: string; description?: string; tags?: string[];
    parameters?: SwaggerParameter[];
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
function isRecord(value:unknown): value is Record<string,unknown> {return !!value && typeof value==='object' && !Array.isArray(value);}
function strings(value:unknown): value is string[] {return Array.isArray(value) && value.every(v=>typeof v==='string');}
export function validateSwagger2(value: unknown): SwaggerSpec {
    const invalid=()=>{throw new Error('Invalid Swagger 2 specification');};
    if(!isRecord(value) || value.swagger!=='2.0' || !isRecord(value.paths))return invalid();
    if(value.info!==undefined && (!isRecord(value.info) || ['title','description','version'].some(k=>value.info && isRecord(value.info) && value.info[k]!==undefined && typeof value.info[k]!=='string')))return invalid();
    if(value.definitions!==undefined && !isRecord(value.definitions))return invalid();
    const definitions=isRecord(value.definitions)?value.definitions:{};
    let nodes=0;
    const schema=(v:unknown,depth=0):void=>{
        if(!isRecord(v) || depth>32 || ++nodes>10000)return invalid();
        for(const key of ['type','format','description','$ref'])if(v[key]!==undefined && typeof v[key]!=='string')return invalid();
        if(v.$ref!==undefined){const ref=v.$ref as string;const prefix='#/definitions/';const key=ref.slice(prefix.length).replace(/~1/g,'/').replace(/~0/g,'~');if(!ref.startsWith(prefix)||!Object.prototype.hasOwnProperty.call(definitions,key))return invalid();}
        if(v.required!==undefined && !strings(v.required))return invalid();
        if(v.enum!==undefined && !Array.isArray(v.enum))return invalid();
        if(v.properties!==undefined){if(!isRecord(v.properties))return invalid();for(const child of Object.values(v.properties))schema(child,depth+1);}
        if(v.items!==undefined)schema(v.items,depth+1);
        if(v.allOf!==undefined){if(!Array.isArray(v.allOf))return invalid();for(const child of v.allOf)schema(child,depth+1);}
        if(v.additionalProperties!==undefined && typeof v.additionalProperties!=='boolean')schema(v.additionalProperties,depth+1);
    };
    for(const definition of Object.values(definitions))schema(definition);
    for(const methods of Object.values(value.paths)) {
        if(!isRecord(methods))return invalid();
        for(const [method,op] of Object.entries(methods)) {
            if(!['get','post','put','patch','delete','head','options'].includes(method))continue;
            if(!isRecord(op))return invalid();
            for(const key of ['summary','description','x-feature-prerequisites'])if(op[key]!==undefined && typeof op[key]!=='string')return invalid();
            if(op.tags!==undefined && !strings(op.tags))return invalid();
            if(op.parameters!==undefined){if(!Array.isArray(op.parameters))return invalid();for(const p of op.parameters){if(!isRecord(p)||typeof p.name!=='string'||typeof p.in!=='string'||(p.required!==undefined && typeof p.required!=='boolean'))return invalid();if(p.schema!==undefined)schema(p.schema);if(p.items!==undefined)schema(p.items);}}
            if(!isRecord(op.responses)||!Object.keys(op.responses).length)return invalid();
            for(const response of Object.values(op.responses)){if(!isRecord(response)||(response.description!==undefined && typeof response.description!=='string'))return invalid();if(response.schema!==undefined)schema(response.schema);}
            if(op.security!==undefined){if(!Array.isArray(op.security))return invalid();for(const entry of op.security){if(!isRecord(entry)||Object.values(entry).some(v=>!strings(v)))return invalid();}}
        }
    }
    return value as unknown as SwaggerSpec;
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
