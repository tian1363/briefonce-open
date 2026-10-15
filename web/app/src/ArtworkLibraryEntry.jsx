import { useEffect, useState } from "react";
import { ChevronRight, Images } from "lucide-react";
import { listArtworks } from "./api.js";

export default function ArtworkLibraryEntry({productId, refreshKey, onOpen}) {
  const [items,setItems]=useState([]);
  useEffect(()=>{
    let active=true;
    setItems([]);
    async function refresh(){try {const result=await listArtworks(productId);if(active)setItems((result.items || []).slice(0,4));} catch {if(active)setItems([]);}}
    refresh();const timer=setInterval(refresh,30000);
    return ()=>{active=false;clearInterval(timer);};
  },[productId,refreshKey]);
  return <div className="workflow-artwork-entry"><span className="product-picker-label">作品库</span><button type="button" onClick={onOpen} aria-label="打开作品库"><span className="workflow-artwork-stack" aria-hidden="true">{items.length ? items.map((item,index)=><img key={item.id} src={item.url} alt="" style={{'--stack-index':index}} />) : <Images size={24}/>}</span><span className="workflow-artwork-copy"><strong>历史生成</strong><small>{items.length ? "查看全部作品" : "暂无历史作品"}</small></span><ChevronRight size={17} aria-hidden="true" /></button></div>;
}
