import skillsCatalogData from '@/lib/generated/skillsCatalog.json';

export type SkillCatalogEntry = {
  name: string;
  title: string;
  description: string;
  markdown: string;
  sourcePath: string;
};

export type SkillsCatalog = {
  generatedAt?: string;
  skills: SkillCatalogEntry[];
};

export const skillsCatalog = skillsCatalogData as SkillsCatalog;
