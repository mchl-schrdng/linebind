import { cleanName } from "../shared/name.mjs";
export const formatProfile = (profile) => ({ name: cleanName(profile.name) });
