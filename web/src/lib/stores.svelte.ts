import { goto } from '$app/navigation';
import { base } from '$app/paths';
import { getCsrfToken } from '$lib/api';

export interface AuthUser {
	id: string;
	email: string;
	name: string;
	role: string;
	force_password_change: boolean;
	totp_enabled: boolean;
	webauthn_enabled: boolean;
	enforce_2fa: boolean;
	password_expired: boolean;
}

function createAuthStore() {
	let user = $state<AuthUser | null>(null);
	let checked = $state(false);

	async function initialize() {
		try {
			const res = await fetch('/admin/api/me', { credentials: 'same-origin' });
			if (!res.ok) {
				user = null;
				return;
			}
			user = await res.json();
		} catch {
			user = null;
		} finally {
			checked = true;
		}
	}

	function setUser(newUser: AuthUser) {
		user = newUser;
		checked = true;
	}

	async function logout() {
		try {
			await fetch('/admin/api/logout', {
				method: 'POST',
				headers: { 'X-CSRF-Token': getCsrfToken() ?? '' },
				credentials: 'same-origin'
			});
		} catch {
			// Best-effort
		}
		user = null;
		checked = true;
		goto(`${base}/login`);
	}

	return {
		get user() {
			return user;
		},
		get isAuthenticated() {
			return !!user;
		},
		get isAdmin() {
			return user?.role === 'admin';
		},
		get checked() {
			return checked;
		},
		initialize,
		setUser,
		logout
	};
}

export const auth = createAuthStore();
