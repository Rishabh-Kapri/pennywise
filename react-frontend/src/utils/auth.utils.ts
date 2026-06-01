type JWTPayload = {
  exp?: number;
};

export function parseJWT(jwtString: string): JWTPayload | null {
  try {
    const split = jwtString.split('.');
    if (split.length !== 3) {
      return null;
    }

    const payload = split[1].replace(/-/g, '+').replace(/_/g, '/');
    const paddedPayload = payload.padEnd(payload.length + ((4 - (payload.length % 4)) % 4), '=');
    return JSON.parse(atob(paddedPayload)) as JWTPayload;
  } catch (err) {
    console.error(err);
    return null;
  }
}
