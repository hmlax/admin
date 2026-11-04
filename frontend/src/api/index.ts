// Public HTTP clients and application query helpers.
export * from './generated/admin/v1/index';

/** export all API functions for easier imports in other modules */
export * from './hooks';

// Hooks expose application types that refine the generated transport values.
export type { AiConversation } from './hooks/ai-chat';
export type { AiDoc, AiKnowledgeBase } from './hooks/ai-knowledge';
export type { AiQueryRow } from './hooks/ai-query';
