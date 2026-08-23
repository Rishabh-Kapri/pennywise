/** Mirrors sharedModel.AuthProviderUserResponse (backend/shared/model/auth.go). */
export interface ConnectedProvider {
  providerType: string;
  providerId: string;
  oauthClientType?: string;
  email?: string;
  name?: string;
  picture?: string;
  gmailHistoryId?: number;
  lastGmailSync?: string;
  expiryAt?: number;
  verifiedAt?: string;
}

/** Mirrors sharedModel.CurrentAuthUserResponse. */
export interface CurrentUser {
  id: string;
  email: string;
  name: string;
  picture?: string;
  createdAt?: string;
  updatedAt?: string;
  providers: ConnectedProvider[];
}
