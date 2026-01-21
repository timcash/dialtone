import fs from 'fs';
import path from 'path';

const docsDirectory = path.join(process.cwd(), 'content/docs');

export function getAllDocsSlugs() {
  // If the directory doesn't exist (e.g. in some build environments), return empty array
  if (!fs.existsSync(docsDirectory)) {
    console.warn(`Docs directory not found: ${docsDirectory}`);
    return [];
  }

  const fileNames = fs.readdirSync(docsDirectory);
  return fileNames.filter(fileName => fileName.endsWith('.md')).map((fileName) => {
    return {
      slug: fileName.replace(/\.md$/, ''),
    };
  });
}

export function getDocData(slug: string) {
  const fullPath = path.join(docsDirectory, `${slug}.md`);

  if (!fs.existsSync(fullPath)) {
    return null;
  }

  const fileContents = fs.readFileSync(fullPath, 'utf8');
  return {
    slug,
    content: fileContents,
  };
}
