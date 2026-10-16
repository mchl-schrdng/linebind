import { formatProfile } from "../domain/profile.mjs";

export const renderName = () => formatProfile({ name: "Ada" }).name;
