// API client for Skill Registry
const API_BASE = '/api/v1';

export async function fetchSkills(params = {}) {
  const query = new URLSearchParams();
  if (params.q) query.append('q', params.q);
  if (params.namespace) query.append('namespace', params.namespace);
  if (params.tag) query.append('tag', params.tag);
  if (params.limit) query.append('limit', params.limit);
  if (params.offset) query.append('offset', params.offset);
  
  const url = `${API_BASE}/skills?${query}`;
  const response = await fetch(url);
  if (!response.ok) throw new Error('Failed to fetch skills');
  return response.json();
}

export async function fetchSkillDetails(namespace, name) {
  const response = await fetch(`${API_BASE}/skills/${namespace}/${name}`);
  if (!response.ok) throw new Error('Failed to fetch skill details');
  return response.json();
}

export async function fetchVersionDetails(namespace, name, version) {
  const response = await fetch(`${API_BASE}/skills/${namespace}/${name}/versions/${version}`);
  if (!response.ok) throw new Error('Failed to fetch version details');
  return response.json();
}

export async function getDownloadUrl(namespace, name, version) {
  return `${API_BASE}/skills/${namespace}/${name}/versions/${version}/download`;
}

export async function fetchMetadata() {
  const response = await fetch(`${API_BASE}/metadata`);
  if (!response.ok) throw new Error('Failed to fetch metadata');
  return response.json();
}
