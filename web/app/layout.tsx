import type { ReactNode } from "react";
import "./style.css";
export const metadata = {title:"Veltryx Vision",description:"Prototype OCR racing"};
export default function Layout({children}:{children:ReactNode}){return <html lang="fr"><body>{children}</body></html>}
