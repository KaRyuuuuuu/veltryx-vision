import {NextRequest, NextResponse} from "next/server";
export const dynamic = "force-dynamic";
export async function GET(_req:NextRequest,{params}:{params:Promise<{path:string[]}>}) {
 const {path}=await params;
 if(!path.length||!["status","events","frame.jpg","info","captures"].includes(path[0])||path.some(p=>!p||p==="."||p===".."||p.includes("/")||p.includes("\\")))return NextResponse.json({error:"bad path"},{status:400});
 try {
  const upstream=await fetch("http://127.0.0.1:8098/api/"+path.map(encodeURIComponent).join("/"),{cache:"no-store",signal:AbortSignal.timeout(5000)});
  return new NextResponse(await upstream.arrayBuffer(),{status:upstream.status,headers:{"Content-Type":upstream.headers.get("content-type")||"application/octet-stream","Cache-Control":"no-store"}});
 }catch{return NextResponse.json({error:"Go service offline"},{status:502});}
}
