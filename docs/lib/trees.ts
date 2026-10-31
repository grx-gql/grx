import type { Folder, Node, Root } from 'fumadocs-core/page-tree';
import { source } from './source';

export const referenceRoute = '/reference';

const isOverview = (node: Node) => node.type === 'page' && node.url === referenceRoute;

// The overview is the folder index, or its first child when meta.json lists "index" explicitly.
function isReference(node: Node): node is Folder {
  return node.type === 'folder' && (node.index?.url === referenceRoute || node.children.some(isOverview));
}

export function isReferenceSlug(slugs: string[] | undefined) {
  return slugs?.[0] === referenceRoute.slice(1);
}

/** The guide sidebar: everything except the API reference, which has its own section. */
export function docsTree(): Root {
  const tree = source.getPageTree();
  return { ...tree, children: tree.children.filter((node) => !isReference(node)) };
}

/** The API reference sidebar: the reference overview plus one entry per package. */
export function referenceTree(): Root {
  const tree = source.getPageTree();
  const folder = tree.children.find(isReference);
  if (!folder) return { name: 'API reference', children: [] };
  const children = folder.children.map((node) => (isOverview(node) ? { ...node, name: 'Overview' } : node));
  return {
    name: 'API reference',
    children: folder.index ? [{ ...folder.index, name: 'Overview' }, ...children] : children,
  };
}
