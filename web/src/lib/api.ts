import { goto } from '$app/navigation';
import { base } from '$app/paths';

const API_BASE = '/admin/api';

interface ApiError {
	status: number;
	message: string;
}

class ApiRequestError extends Error {
	status: number;

	constructor(status: number, message: string) {
		super(message);
		this.status = status;
		this.name = 'ApiRequestError';
	}
}

export function getCsrfToken(): string | null {
	const match = document.cookie.match(/(?:^|;\s*)lmgate_csrf=([^;]*)/);
	return match ? decodeURIComponent(match[1]) : null;
}

export async function initCsrf(): Promise<void> {
	if (getCsrfToken()) return;
	await fetch(`${API_BASE}/csrf-token`, { method: 'GET', credentials: 'same-origin' });
}

function buildHeaders(method: string, extra?: HeadersInit): Headers {
	const headers = new Headers(extra);

	if (!headers.has('Content-Type')) {
		headers.set('Content-Type', 'application/json');
	}

	if (method !== 'GET' && method !== 'HEAD') {
		const csrf = getCsrfToken();
		if (csrf) {
			headers.set('X-CSRF-Token', csrf);
		}
	}

	return headers;
}

async function handleResponse<T>(response: Response): Promise<T> {
	if (response.status === 401) {
		await goto(`${base}/login`);
		throw new ApiRequestError(401, 'Unauthorized');
	}

	if (!response.ok) {
		const body = await response.text();
		let message = `Request failed with status ${response.status}`;
		try {
			const parsed = JSON.parse(body);
			if (parsed.error) message = parsed.error;
			if (parsed.message) message = parsed.message;
		} catch {
			// body is not JSON, use default message
		}
		throw new ApiRequestError(response.status, message);
	}

	if (response.status === 204) {
		return undefined as T;
	}

	return response.json() as Promise<T>;
}

export async function get<T>(path: string): Promise<T> {
	const response = await fetch(`${API_BASE}${path}`, {
		method: 'GET',
		headers: buildHeaders('GET'),
		credentials: 'same-origin'
	});
	return handleResponse<T>(response);
}

export async function post<T>(path: string, body?: unknown): Promise<T> {
	const response = await fetch(`${API_BASE}${path}`, {
		method: 'POST',
		headers: buildHeaders('POST'),
		body: body ? JSON.stringify(body) : undefined,
		credentials: 'same-origin'
	});
	return handleResponse<T>(response);
}

export async function put<T>(path: string, body?: unknown): Promise<T> {
	const response = await fetch(`${API_BASE}${path}`, {
		method: 'PUT',
		headers: buildHeaders('PUT'),
		body: body ? JSON.stringify(body) : undefined,
		credentials: 'same-origin'
	});
	return handleResponse<T>(response);
}

export async function del<T>(path: string): Promise<T> {
	const response = await fetch(`${API_BASE}${path}`, {
		method: 'DELETE',
		headers: buildHeaders('DELETE'),
		credentials: 'same-origin'
	});
	return handleResponse<T>(response);
}

export async function streamPost(path: string, body?: unknown): Promise<Response> {
	const response = await fetch(`${API_BASE}${path}`, {
		method: 'POST',
		headers: buildHeaders('POST'),
		body: body ? JSON.stringify(body) : undefined,
		credentials: 'same-origin'
	});

	if (response.status === 401) {
		await goto(`${base}/login`);
		throw new ApiRequestError(401, 'Unauthorized');
	}

	if (!response.ok) {
		const text = await response.text();
		let message = `Request failed with status ${response.status}`;
		try {
			const parsed = JSON.parse(text);
			if (parsed.error) message = parsed.error;
		} catch {
			// not JSON
		}
		throw new ApiRequestError(response.status, message);
	}

	return response;
}

export { ApiRequestError };
export type { ApiError };
