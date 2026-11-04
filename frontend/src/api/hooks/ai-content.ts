import { useMutation } from '@tanstack/react-query';
import { apiClient } from '@/api/client';

export interface GenerateContentParams {
  scene: NonNullable<Parameters<typeof apiClient.aiContentService.GenerateContent>[0]['scene']>;
  topic: string;
  context?: string;
  lang?: string;
  maxLength?: number;
}

// useGenerateContent — AI 内容生成（表单助手：按场景生成文本填入字段）
export function useGenerateContent() {
  return useMutation({
    mutationFn: (req: GenerateContentParams) =>
      apiClient.aiContentService.GenerateContent({
        scene: req.scene,
        topic: req.topic,
        context: req.context,
        lang: req.lang ?? 'zh-CN',
        maxLength: req.maxLength,
      }),
  });
}
