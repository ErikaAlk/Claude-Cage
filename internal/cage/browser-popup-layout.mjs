import { readFileSync } from 'node:fs';

// 原生界面端口只允许此服务的独立用户连接，由启动前的防火墙保护。
const socket = new WebSocket('ws://127.0.0.1:19223/0');
const pending = new Map();
let sequence = 0, queued = false, running = false, dirty = false;
let document;
const nodes = new Map();
const attributes = node => Object.fromEntries(Array.from({ length: (node.attributes?.length || 0) / 2 }, (_, index) => [node.attributes[index * 2], node.attributes[index * 2 + 1]]));
function call(method, params = {}, elementId) {
  if (elementId && !nodes.has(elementId)) return Promise.resolve(null);
  return new Promise((resolve, reject) => {
    const id = ++sequence;
    const timer = setTimeout(() => { pending.delete(id); reject(new Error('原生弹窗布局接口超时')); }, 4000);
    pending.set(id, { resolve, reject, timer, elementId });
    socket.send(JSON.stringify({ id, method, params }));
  });
}
async function style(node) {
  const result = await call('CSS.getMatchedStylesForNode', { nodeId: node.nodeId }, node.nodeId);
  if (!result) return null;
  const rule = result.matchedCSSRules.find(item => item.rule.style.cssProperties.some(property => property.name === 'Width'))?.rule;
  if (!rule) throw new Error('原生弹窗尺寸属性缺失');
  return { id: rule.styleSheetId, values: Object.fromEntries(rule.style.cssProperties.map(property => [property.name, property.value])) };
}
async function refresh() {
  if (!document) return;
  for (const branch of document.children || []) await position(branch);
}
async function position(branch) {
  const popups = [];
  let browser;
  function visit(node) {
    const properties = attributes(node);
    if (properties.class === 'BrowserRootView') browser = node;
    if (node.nodeName === 'Widget' && properties.name === 'ExtensionPopup') popups.push(node);
    for (const child of node.children || []) visit(child);
  }
  visit(branch);
  if (!browser || !popups.length) return;
  const browserStyle = await style(browser);
  if (!browserStyle) return;
  const { values: viewport } = browserStyle;
  const width = Number(viewport.Width);
  if (!Number.isFinite(width) || width <= 0) throw new Error('浏览器原生窗口宽度无效');
  const anchor = JSON.parse(readFileSync('/run/cage-native-ui/extension-anchor.json', 'utf8'));
  if (!Number.isFinite(anchor.right) || anchor.right < 0 || anchor.right > 1) throw new Error('扩展按钮位置无效');
  for (const popup of popups) {
    const popupStyle = await style(popup);
    if (!popupStyle) continue;
    const { id, values } = popupStyle;
    if (values.Visible !== 'true') continue;
    const popupWidth = Number(values.Width);
    if (![popupWidth, Number(values.X), Number(values.Y)].every(Number.isFinite) || popupWidth <= 0) throw new Error('原生弹窗位置或尺寸无效');
    const x = Math.max(0, Math.min(width - popupWidth, Math.round(width * anchor.right) - popupWidth));
    const y = 0;
    if (Number(values.X) === x && Number(values.Y) === y) continue;
    await call('CSS.setStyleTexts', { edits: [{ styleSheetId: id, text: `X: ${x}; Y: ${y};`, range: { startLine: 0, startColumn: 0, endLine: 0, endColumn: 0 } }] }, popup.nodeId);
  }
}
function schedule() {
  dirty = true;
  if (queued || running) return;
  queued = true;
  setImmediate(async () => {
    queued = false;
    running = true;
    do { dirty = false; await refresh(); } while (dirty);
    running = false;
  });
}
socket.onmessage = event => {
  const message = JSON.parse(event.data);
  if (message.id) {
    const request = pending.get(message.id);
    if (!request) throw new Error('原生弹窗布局响应无效');
    pending.delete(message.id);
    clearTimeout(request.timer);
    if (message.error && request.elementId && !nodes.has(request.elementId)) request.resolve(null);
    else if (message.error) request.reject(new Error(message.error.message));
    else request.resolve(message.result);
  } else if (message.method === 'DOM.childNodeInserted' && document) {
    const parent = nodes.get(message.params.parentNodeId);
    if (!parent) return;
    const previous = parent.children.findIndex(child => child.nodeId === message.params.previousNodeId);
    parent.children.splice(previous + 1, 0, message.params.node);
    index(message.params.node);
    schedule();
  } else if (message.method === 'DOM.childNodeRemoved' && document) {
    const parent = nodes.get(message.params.parentNodeId);
    if (!parent) return;
    const removed = nodes.get(message.params.nodeId);
    if (!removed) return;
    function remove(node) { nodes.delete(node.nodeId); for (const child of node.children || []) remove(child); }
    remove(removed);
    parent.children = parent.children.filter(child => child.nodeId !== message.params.nodeId);
    schedule();
  } else if (message.method === 'CSS.styleSheetChanged') schedule();
};
socket.onerror = () => { throw new Error('原生弹窗布局连接失败'); };
socket.onclose = () => { throw new Error('原生弹窗布局连接已关闭'); };
function index(node) { nodes.set(node.nodeId, node); node.children ||= []; for (const child of node.children) index(child); }
socket.onopen = async () => {
  await call('CSS.enable');
  document = (await call('DOM.getDocument')).root;
  index(document);
  schedule();
};
