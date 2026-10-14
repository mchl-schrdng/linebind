import { getProfile } from "../api/profile.mjs";

export const renderName = () => getProfile().name;
