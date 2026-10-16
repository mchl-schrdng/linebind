import { formatProfile } from "../domain/profile.mjs";
import { loadProfile } from "../adapters/profile.mjs";

export const getProfile = () => formatProfile(loadProfile());
