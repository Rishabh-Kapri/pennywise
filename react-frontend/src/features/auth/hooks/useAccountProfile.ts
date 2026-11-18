import { useEffect, useState } from 'react';
import { apiClient } from '@/utils';

/** A sign-in method linked to the account; one per app (web, Android). */
export interface ConnectedProvider {
  providerType: string;
  providerId: string;
  email?: string;
  name?: string;
  picture?: string;
}

/** The signed-in account with its sign-in methods (`GET auth/users/me`). */
export interface AccountProfile {
  id: string;
  email: string;
  name: string;
  picture?: string;
  providers: ConnectedProvider[];
}

/** Loads the signed-in account's profile; null until it arrives or if it fails. */
export function useAccountProfile(): AccountProfile | null {
  const [profile, setProfile] = useState<AccountProfile | null>(null);
  useEffect(() => {
    let live = true;
    apiClient
      .get<AccountProfile>('auth/users/me')
      .then((data) => live && setProfile(data))
      .catch(() => live && setProfile(null));
    return () => {
      live = false;
    };
  }, []);
  return profile;
}
