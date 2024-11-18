"use strict";(self.webpackChunk=self.webpackChunk||[]).push([[3985],{99861:function(We,Fe,u){var De=u(15009),P=u.n(De),Ze=u(64599),W=u.n(Ze),Ae=u(99289),U=u.n(Ae),ye=u(5574),x=u.n(ye),Ie=u(42119),K=u(67294),f=u(92754),L=u(3393),be=u(184),Ee=u(38345),G=u(85893),d=(0,K.forwardRef)(function(ue,Re){(0,K.useImperativeHandle)(Re,function(){return{start:function(){me(!0)}}});var te=(0,K.useState)(0),se=x()(te,2),Y=se[0],je=se[1],z=(0,K.useRef)(),de=(0,K.useState)(!1),ce=x()(de,2),ne=ce[0],me=ce[1];return(0,G.jsxs)(be.a,{trigger:ue.trigger,width:800,open:ne,submitter:!1,title:"\u4E00\u952E\u62C9\u53D6\u955C\u50CF",onOpenChange:function(){var J=U()(P()().mark(function ae(Ce){var ve,re,Q,ge,le,fe,e;return P()().wrap(function(g){for(;;)switch(g.prev=g.next){case 0:if(me(Ce),!Ce){g.next=26;break}re=0,Q=W()(ue.image),g.prev=4,Q.s();case 6:if((ge=Q.n()).done){g.next=16;break}return fe=ge.value,je(re),(le=z.current)===null||le===void 0||le.setStart(fe),g.next=12,(0,L.Gb)({tag:fe,type:"pull"});case 12:e=g.sent,re++;case 14:g.next=6;break;case 16:g.next=21;break;case 18:g.prev=18,g.t0=g.catch(4),Q.e(g.t0);case 21:return g.prev=21,Q.f(),g.finish(21);case 24:(ve=z.current)===null||ve===void 0||ve.setFinish(),je(re);case 26:case"end":return g.stop()}},ae,null,[[4,18,21,24]])}));return function(ae){return J.apply(this,arguments)}}(),children:[(0,G.jsx)(Ee.Z,{children:(0,G.jsx)(Ie.Z,{current:Y,items:ue.image.map(function(J,ae){return{title:J,key:ae}})})}),(0,G.jsx)(Ee.Z,{children:(0,G.jsx)(f.Z,{ref:z})})]})});Fe.Z=d},27381:function(We,Fe,u){u.r(Fe),u.d(Fe,{default:function(){return fu}});var De=u(15009),P=u.n(De),Ze=u(99289),W=u.n(Ze),Ae=u(5574),U=u.n(Ae),ye=u(90078),x=u(38345),Ie=u(97269),K=u(2236),f=u(5966),L=u(97462),be=u(64218),Ee=u(92398),G=u(38925),d=u(67294),ue=u(35880),Re=u(10641),te=u(62370),se=u(85576),Y=u(42075),je=u(96074),z=u(25593),de=u(83062),ce=u(71230),ne=u(15746),me=u(14726),J=u(3393),ae=u(18148),Ce=u(5251),ve=u(64789),re=u(75162),Q=u(23430),ge=u(28307),le=u(78451),fe=u(99861),e=u(85893);function Se(r){var y=(0,d.useState)(!1),B=U()(y,2),I=B[0],i=B[1],h=(0,d.useRef)(),l=(0,d.useContext)(ue.Z),o=l.createFormRef,C=l.volumeListRef,D=l.createEnvRef,p=l.domainRef,E=(0,d.useRef)(),V=function(){var c=W()(P()().mark(function m(a){var j,b,R,s,T,N,Z,ie,S,O,q,w;return P()().wrap(function(n){for(;;)switch(n.prev=n.next){case 0:return i(!1),n.next=3,(0,J.YU)({md5:a});case 3:s=n.sent,r.redeploy||(ie=(T=o.current)===null||T===void 0?void 0:T.getFieldsValue(),(N=o.current)===null||N===void 0||N.resetFields(),(Z=o.current)===null||Z===void 0||Z.setFieldsValue({siteName:ie.siteName,siteTitle:ie.siteTitle})),s.data.info.Config.Env&&s.data.info.Config.Env.map(function(t){var k,M=t.split("=");(k=D.current)===null||k===void 0||k.addEnvItem(M[0],M.slice(1).join("="))}),(j=o.current)===null||j===void 0||j.setFieldValue("imageName",a),(b=o.current)===null||b===void 0||b.setFieldValue("workDir",s.data.info.Config.WorkingDir),(R=p.current)===null||R===void 0||R.setExposePort(s.data.info.Config.ExposedPorts),r.redeploy||(s.data.info.Config.Volumes?(q=[],Object.keys(s.data.info.Config.Volumes).map(function(t,k){q.push(t)}),(O=C.current)===null||O===void 0||O.setDefaultDestPath(q)):(w=C.current)===null||w===void 0||w.setDefaultDestPath([]),(S=o.current)===null||S===void 0||S.setFieldsValue({workDir:{value:"",useDefault:!0,default:s.data.info.Config.WorkingDir},user:{value:"",useDefault:!0,default:s.data.info.Config.User},command:{value:"",useDefault:!0,default:s.data.info.Config.Cmd&&s.data.info.Config.Cmd.join(" ")},entrypoint:{value:"",useDefault:!0,default:s.data.info.Config.Entrypoint&&s.data.info.Config.Entrypoint.join(" ")}}));case 10:case"end":return n.stop()}},m)}));return function(a){return c.apply(this,arguments)}}();return(0,d.useEffect)(function(){r.fromImageId&&V(r.fromImageId)},[r.fromImageId]),(0,e.jsxs)(e.Fragment,{children:[(0,e.jsx)(se.Z,{open:I,width:1024,title:"\u9009\u62E9\u955C\u50CF",footer:!1,onCancel:function(){return i(!1)},children:(0,e.jsx)(Re.Z,{scroll:{x:"max-content"},columns:[{title:"Id",dataIndex:"Id",search:!1,width:200,render:function(m,a,j,b,R){return(0,e.jsx)(le.Z,{content:a.Id})}},{title:"\u955C\u50CF\u540D\u79F0",dataIndex:"tag",width:200,render:function(m,a,j,b){return(0,e.jsx)(le.Z,{content:a.RepoTags[0]})}},{title:"\u521B\u5EFA\u65E5\u671F",dataIndex:"Created",ellipsis:!0,search:!1,width:"180px",render:function(m,a,j,b){return(0,e.jsx)("div",{children:(0,Ce.ZM)(a.Created*1e3)},a.Id)},sorter:function(m,a){return m.Created-a.Created}},{title:"\u64CD\u4F5C",valueType:"option",key:"option",width:80,render:function(m,a,j,b){return(0,e.jsxs)(Y.Z,{split:(0,e.jsx)(je.Z,{type:"vertical"}),children:[(0,e.jsx)(z.Z.Link,{onClick:W()(P()().mark(function R(){return P()().wrap(function(T){for(;;)switch(T.prev=T.next){case 0:return T.abrupt("return",V(a.RepoTags[0]));case 1:case"end":return T.stop()}},R)})),children:(0,e.jsx)(de.Z,{title:"\u4F7F\u7528\u955C\u50CF",children:(0,e.jsx)(ve.Z,{})})},"addImage"),(0,e.jsx)(z.Z.Link,{onClick:W()(P()().mark(function R(){var s;return P()().wrap(function(N){for(;;)switch(N.prev=N.next){case 0:a.RepoTags[0]&&((s=E.current)===null||s===void 0||s.setImageName(a.RepoTags[0]));case 1:case"end":return N.stop()}},R)})),children:(0,e.jsx)(de.Z,{title:"\u66F4\u65B0\u955C\u50CF",children:(0,e.jsx)(re.Z,{})})},"updateImage")]})}}],request:function(){var c=W()(P()().mark(function m(a,j,b){var R,s,T,N,Z;return P()().wrap(function(S){for(;;)switch(S.prev=S.next){case 0:return r.redeploy&&(s=(R=o.current)===null||R===void 0?void 0:R.getFieldValue("imageName"),a.tag=s?s.split(":")[0]:a.tag),S.next=3,(0,ae.KG)({tag:a.tag});case 3:return T=S.sent,N=0,Z=[],T.data.list&&(Z=T.data.list.flatMap(function(O){return O.RepoTags.map(function(q){return{Key:N++,Id:O.Id,Created:O.Created,RepoTags:[q]}})})),S.abrupt("return",{data:Z,success:!0,total:Z.length});case 8:case"end":return S.stop()}},m)}));return function(m,a,j){return c.apply(this,arguments)}}(),toolBarRender:function(){return[(0,e.jsx)(ge.Z,{onClick:function(){i(!1)},buttonType:"primary",ref:E,onFinish:function(a){V(a)}},"remote")]},formRef:h,rowKey:"Key",pagination:{pageSize:5}})},"imageTableList"),(0,e.jsxs)(ce.Z,{children:[(0,e.jsx)(ne.Z,{span:14,children:(0,e.jsx)(f.Z,{label:"\u955C\u50CF",tooltip:r.redeploy?"\u66F4\u65B0\u5BB9\u5668\u65F6,\u53EA\u53EF\u4EE5\u9009\u62E9\u8BE5\u955C\u50CF\u4E0B\u7684\u4E0D\u540Ctag.\u4E5F\u53EF\u4EE5\u5F3A\u5236\u66F4\u65B0\u5DF2\u90E8\u7F72tag,\u66F4\u6B21\u90E8\u7F72":"\u65B0\u5EFA\u5BB9\u5668\u65F6,\u53EF\u4EE5\u4F7F\u7528\u672C\u5730\u955C\u50CF,\u4E5F\u53EF\u4EE5\u4E0B\u8F7D\u5168\u65B0\u7684\u955C\u50CF\u6216\u662F\u66F4\u65B0\u672C\u5730\u955C\u50CF\u7684tag",name:"imageName",disabled:!0,placeholder:"\u8BF7\u9009\u62E9\u955C\u50CF\uFF0C\u5982\u679C\u662F\u8FDC\u7A0B\u955C\u50CF\u8BF7\u5148\u4E0B\u8F7D",rules:[{required:!0}],required:!0})}),(0,e.jsx)(ne.Z,{children:(0,e.jsx)(te.Z,{label:" ",children:(0,e.jsxs)(Y.Z,{children:[(0,e.jsx)(me.ZP,{type:"primary",onClick:function(){return i(!0)},children:r.redeploy?"\u66F4\u65B0\u955C\u50CF":"\u9009\u62E9\u955C\u50CF"},"showBtn"),(0,e.jsx)(L.Z,{name:["imageName"],children:function(m){var a=m.imageName;if(r.redeploy)return(0,e.jsx)(fe.Z,{image:[a],trigger:(0,e.jsx)(de.Z,{title:"\u5FEB\u901F\u91CD\u65B0\u62C9\u53D6\u955C\u50CF\uFF0C\u66F4\u65B0\u5BB9\u5668",children:(0,e.jsx)(me.ZP,{icon:(0,e.jsx)(Q.Z,{}),children:"\u66F4\u65B0\u6216\u662F\u62C9\u53D6\u5F53\u524D\u955C\u50CF"})})})}})]})})})]})]})}var g=u(60335),$e=u(24969),X=u(24739),Ke=u(63434),Pe=u(17186),Ge=u(92067),Ye=(0,d.forwardRef)(function(r,y){var B=(0,d.useState)(!1),I=U()(B,2),i=I[0],h=I[1],l=(0,d.useRef)(),o=function(p){var E=(0,d.useState)([]),V=U()(E,2),c=V[0],m=V[1];return(0,d.useEffect)(function(){(0,g.jV)({md5:p.name}).then(function(a){var j;return m((j=a.data.info.Config.Env)!==null&&j!==void 0?j:[]),!0})},[]),(0,e.jsxs)(x.Z,{bordered:!0,extra:p.action,style:{marginBottom:10},children:[(0,e.jsx)(x.Z,{title:"\u5173\u8054\u4FE1\u606F",children:(0,e.jsxs)(X.UW,{children:[(0,e.jsx)(f.Z,{label:"\u5BB9\u5668\u540D\u79F0",name:"name",width:"md",readonly:!0}),(0,e.jsx)(f.Z,{label:"\u8BBF\u95EEHostName",name:"alise",width:"md",tooltip:"\u901A\u8FC7\u914D\u7F6E\u6B64\u540D\u79F0\uFF0C\u5728\u5BB9\u5668\u5185\u90E8\u8FDB\u884C\u8BF7\u6C42\u8BBF\u95EE"}),(0,e.jsx)(Ke.Z,{label:"\u5173\u8054\u5B58\u50A8",name:"volume"})]})}),(0,e.jsx)(x.Z,{title:"\u73AF\u5883\u53D8\u91CF",children:(0,e.jsx)(Y.Z,{direction:"vertical",children:c&&c.map(function(a,j){return(0,e.jsx)(z.Z.Text,{copyable:{icon:(0,e.jsx)($e.Z,{}),onCopy:function(){var R=a.split("=");r.onAddEnv(R[0],R[1])},tooltips:["add env","success"]},code:!0,ellipsis:{tooltip:a},style:{width:300},children:a},j)})},"linkEnv")})]})},C=function(p){var E,V,c=!1,m=(E=(V=l.current)===null||V===void 0?void 0:V.getList())!==null&&E!==void 0?E:[];if(m.map(function(j){j.name==p.name&&(c=!0)}),!c){var a;(a=l.current)===null||a===void 0||a.add(p)}};return(0,d.useImperativeHandle)(y,function(){return{setDefaultLink:function(p){p&&p.map(function(E){E.name!=""&&C(E)})}}}),(0,e.jsxs)(x.Z,{title:"\u5173\u8054\u5BB9\u5668",headerBordered:!0,children:[(0,e.jsx)(Pe.u,{name:"links",actionRef:l,creatorButtonProps:{creatorButtonText:"\u6DFB\u52A0\u5173\u8054"},actionGuard:{beforeAddRow:function(p,E){return h(!0),!1}},copyIconProps:!1,min:0,itemRender:function(p,E){return(0,e.jsx)(o,{action:p.action,name:E.record.name})}}),(0,e.jsx)(se.Z,{title:"\u9009\u62E9\u5BB9\u5668",width:1024,footer:!1,open:i,onCancel:function(){return h(!1)},children:(0,e.jsx)(Ge.Z,{onSelect:function(p){C({name:p.Name,alise:p.Config.Hostname,volume:!1}),h(!1)}})})]})}),ze=Ye,Je=u(91058),Qe=u(10523),Xe=u(2831),xe=u(64317),he=u(52688),Le=u(86125),_e=u(44349);function qe(r){var y,B=(0,d.useState)(),I=U()(B,2),i=I[0],h=I[1];return(0,d.useEffect)(function(){(0,Xe.aF)().then(function(l){return h(l.data.info)})},[]),(0,e.jsxs)(x.Z,{children:[(0,e.jsx)(f.Z,{label:"\u5171\u4EAB\u5185\u5B58\u5927\u5C0F - /dev/shm",name:"shmsize",initialValue:"64M",tooltip:"0 \u4E3A\u4F7F\u7528\u9ED8\u8BA464M"}),(0,e.jsx)(te.Z,{label:"\u6700\u5927Cpu\u914D\u989D",name:"cpus",tooltip:"0 \u4E3A\u4E0D\u9650\u5236",children:(0,e.jsx)(Le.Z,{step:.1,max:i==null?void 0:i.NCPU,marks:{0:"0",.5:"0.5\u6838",1:"1\u6838",1.5:"1.5\u6838",2:"2\u6838",4:"4\u6838",6:"6\u6838",8:"8\u6838"}})}),(0,e.jsx)(te.Z,{label:"\u6700\u5927\u5185\u5B58\u914D\u989D",name:"memory",tooltip:"0 \u4E3A\u4E0D\u9650\u5236",children:(0,e.jsx)(Le.Z,{step:256,max:Math.round(((y=i==null?void 0:i.MemTotal)!==null&&y!==void 0?y:0)/1024/1024),marks:{0:"0",1024:"1G",2048:"2G",3072:"3G",4096:"4G"}})}),(0,e.jsx)(xe.Z,{label:"\u65E5\u5FD7\u7C7B\u578B",name:["log","driver"],tooltip:"\u9ED8\u8BA4\u4F7F\u7528 json-file \u9A71\u52A8\u7531 docker \u7EDF\u4E00\u7BA1\u7406\u3002\u91C7\u7528 journal \u65F6\u65E5\u5FD7\u5C06\u6765\u5BBF\u4E3B\u673A\u7BA1\u7406",initialValue:"json-file",valueEnum:{"json-file":"json-file",journald:"journal (\u5BBF\u4E3B\u9700\u8981\u5B89\u88C5journal\u670D\u52A1)"}}),(0,e.jsx)(L.Z,{name:["log"],children:function(o){var C=o.log;if(C.driver=="json-file")return(0,e.jsxs)(X.UW,{children:[(0,e.jsx)(f.Z,{label:"\u65E5\u5FD7\u5207\u5272\u5C3A\u5BF8",name:["log","maxSize"],placeholder:"\u4F8B\u5982\uFF1A10k,5M",tooltip:"\u9ED8\u8BA4 Docker \u5E76\u4E0D\u4F1A\u81EA\u52A8\u5207\u5272\u65E5\u5FD7"}),(0,e.jsx)(f.Z,{tooltip:"\u9ED8\u8BA4 Docker \u5E76\u4E0D\u4F1A\u81EA\u52A8\u6E05\u7406\u65E5\u5FD7\u6587\u4EF6",label:"\u4FDD\u7559\u65E5\u5FD7\u6587\u4EF6\u6570",name:["log","maxFile"],placeholder:"\u4F8B\u5982\uFF1A10"})]})}}),(0,e.jsx)(_e.Z,{name:"device",label:"\u5173\u8054\u8BBE\u5907",hideCopyButton:!0,showAddButton:!0,items:[{label:"\u5BBF\u4E3B\u673A\u8DEF\u5F84",name:"host",width:"md",placeholder:"\u4F8B\u5982\uFF1A/dev/tty0"},{label:"\u5BB9\u5668\u5185\u8DEF\u5F84",name:"dest",width:"md",placeholder:"\u4F8B\u5982\uFF1A/dev/tty0"}]}),(0,e.jsxs)(ce.Z,{children:[(0,e.jsx)(ne.Z,{span:4,children:(0,e.jsx)(he.Z,{label:"\u4F7F\u7528 Gpu",name:["gpus","enable"]})}),(0,e.jsx)(ne.Z,{span:20,children:(0,e.jsx)(L.Z,{name:["gpus"],children:function(o){var C=o.gpus;if(C&&C.enable)return(0,e.jsx)(xe.Z,{mode:"tags",label:"\u8BBE\u5907\u5217\u8868",name:["gpus","device"],placeholder:"\u914D\u7F6E\u4F7F\u7528\u8BBE\u5907id\u6216\u662Fuuid\uFF0C\u4E0D\u586B\u6240\u4E3A\u6240\u6709\u8BBE\u5907"})}})})]}),(0,e.jsx)(L.Z,{name:["gpus"],children:function(o){var C=o.gpus;if(C&&C.enable)return(0,e.jsx)(xe.Z,{label:"\u8BA1\u7B97\u80FD\u529B",name:["gpus","capabilities"],mode:"multiple",fieldProps:{allowClear:!0},options:[{label:"compute - \u8BA1\u7B97\u80FD\u529B",value:"compute"},{label:"compat32 - \u517C\u5BB932\u4F4D",value:"compat32"},{label:"graphics - \u56FE\u5F62\u5904\u7406\u80FD\u529B",value:"graphics"},{label:"utility - \u76D1\u63A7\u548C\u7BA1\u7406\u80FD\u529B",value:"utility"},{label:"video - \u89C6\u9891\u5904\u7406\u80FD\u529B",value:"video"},{label:"display - \u663E\u793A\u56FE\u5F62\u754C\u9762\u80FD\u529B",value:"display"}]})}})]})}function eu(){return(0,e.jsx)(x.Z,{title:"\u5BB9\u5668\u6807\u7B7E",children:(0,e.jsx)(Pe.u,{name:"label",label:"",creatorButtonProps:{creatorButtonText:"\u6DFB\u52A0\u5BB9\u5668\u6807\u7B7E"},copyIconProps:!1,min:0,children:(0,e.jsxs)(X.UW,{children:[(0,e.jsx)(f.Z,{width:"md",name:"name",label:"\u540D\u79F0",placeholder:""}),(0,e.jsx)(f.Z,{width:"md",name:"value",label:"\u503C",placeholder:""})]})})})}var uu=u(44771),tu=u(86615),hu="default",pu="user";function nu(r){return(0,e.jsx)(te.Z,{label:r.label,tooltip:r.tooltip,children:(0,e.jsxs)(Y.Z.Compact,{block:!0,children:[(0,e.jsx)(tu.Z.Group,{radioType:"button",name:[r.name,"useDefault"],options:[{label:"\u4F7F\u7528\u9ED8\u8BA4",value:!0},{label:"\u81EA\u5B9A\u4E49",value:!1}]}),(0,e.jsx)(L.Z,{name:[r.name],children:function(B){return B[r.name]&&B[r.name].useDefault?(0,e.jsx)(f.Z,{name:[r.name,"default"],disabled:!0,placeholder:"\u672A\u8BBE\u7F6E"}):(0,e.jsx)(f.Z,{name:[r.name,"value"]})}})]})})}var Be=nu;function au(){return(0,e.jsx)(e.Fragment,{children:(0,e.jsxs)(x.Z,{children:[(0,e.jsx)(uu.Z,{label:"\u91CD\u542F\u7B56\u7565"}),(0,e.jsxs)(X.UW,{children:[(0,e.jsx)(he.Z,{name:"privileged",label:"\u8D4B\u4E88\u5BB9\u5668Root\u6743\u9650",initialValue:!1}),(0,e.jsx)(he.Z,{name:"autoRemove",label:"\u505C\u6B62\u540E\u81EA\u52A8\u5220\u9664",initialValue:!1})]}),(0,e.jsx)(Be,{label:"\u5DE5\u4F5C\u76EE\u5F55",tooltip:"\u9ED8\u8BA4\u4F7F\u7528\u955C\u50CF\u4E2D\u6307\u5B9A\u7684\u5DE5\u4F5C\u76EE\u5F55",name:"workDir"}),(0,e.jsx)(Be,{label:"User",tooltip:"\u5728\u5BB9\u5668\u4E2D\u8FD0\u884C\u547D\u4EE4\u7684\u7528\u6237",name:"user"}),(0,e.jsx)(Be,{label:"Command",tooltip:"\u542F\u52A8\u5BB9\u5668\u65F6\u8FD0\u884C\u7684\u547D\u4EE4\uFF0C\u4EE5\u7A7A\u683C\u5206\u9694",name:"command"}),(0,e.jsx)(Be,{label:"Entrypoint",tooltip:"\u65E0\u6CD5\u8986\u76D6\u955C\u50CF\u4E2D\u5DF2\u7ECF\u6307\u5B9A\u7684 Entrypoint \u547D\u4EE4",name:"entrypoint"})]})})}var ru=u(91845),Oe=u(62597),Me=u(54006),lu=u(4798),iu=u(82034);function _(r){return(0,e.jsx)("div",{style:{display:r.show?"block":"none"},children:r.children})}function ou(){var r=(0,d.useRef)(),y=function(i){var h,l,o=!1,C=(h=(l=r.current)===null||l===void 0?void 0:l.getList())!==null&&h!==void 0?h:[];if(C.map(function(p){p.name==i.name&&(o=!0)}),!o){var D;(D=r.current)===null||D===void 0||D.add(i)}},B=function(i){var h,l,o=!1,C=(h=(l=r.current)===null||l===void 0?void 0:l.getList())!==null&&h!==void 0?h:[];C.map(function(D,p){if(D.name==i){var E;(E=r.current)===null||E===void 0||E.remove(p)}})};return(0,e.jsx)(x.Z,{title:"\u5173\u8054\u5BBF\u4E3B\u7F51\u7EDC\u4E3B\u673A",tooltip:"\u5BB9\u5668\u5185\u5982\u679C\u9700\u8981\u8BF7\u6C42\u5BBF\u4E3B\u673A\u6240\u5728\u7684\u7F51\u7EDC\u4E2D\u7684\u4E3B\u673A\uFF0C\u53EF\u4EE5\u901A\u8FC7\u6B64\u914D\u7F6E\u5C06Ip\u6CE8\u5165\u5230\u5BB9\u5668\u4E2D",extra:(0,e.jsx)(Y.Z,{children:(0,e.jsx)(he.Z,{name:"enableBindHost",fieldProps:{checkedChildren:"\u7ED1\u5B9A\u5BBF\u4E3B\u673AIp",unCheckedChildren:"\u7ED1\u5B9A\u5BBF\u4E3B\u673AIp",onChange:function(i){i?y({name:"host.dpanel.local",value:"host-gateway"}):B("host.dpanel.local")}}})}),children:(0,e.jsx)(Pe.u,{name:"extraHosts",creatorButtonProps:{creatorButtonText:"\u6DFB\u52A0\u5BBF\u4E3B\u673A\u7F51\u7EDC\u5173\u8054"},actionRef:r,copyIconProps:!1,min:0,children:(0,e.jsxs)(X.UW,{children:[(0,e.jsx)(f.Z,{width:"md",name:"name",label:"Hostname",placeholder:""}),(0,e.jsx)(f.Z,{width:"md",name:"value",label:"ip",placeholder:""})]})})})}var Te=u(90672),su=(0,d.forwardRef)(function(r,y){return(0,d.useImperativeHandle)(y,function(){return{}}),(0,e.jsxs)(x.Z,{title:"\u7F51\u7EDC\u914D\u7F6E",children:[(0,e.jsxs)(X.UW,{children:[(0,e.jsx)(f.Z,{label:"\u914D\u7F6E ipV4 \u5730\u5740",name:["ipV4","address"],width:"md",tooltip:"\u6307\u5B9A\u5BB9\u5668\u7684ipv4\u5730\u5740\uFF0C\u4F8B\u5982 192.168.1.5",placeholder:"192.168.1.5"}),(0,e.jsx)(L.Z,{name:["ipV4"],children:function(I){var i=I.ipV4;return(0,e.jsx)(f.Z,{label:"\u914D\u7F6E ipV4 \u5B50\u7F51",name:["ipV4","subnet"],width:"md",required:i&&i.address,tooltip:"\u6307\u5B9A\u5BB9\u5668\u7684ipv4\u6240\u5728\u7684\u5B50\u7F51\uFF0C\u4F8B\u5982 192.168.1.0/24",placeholder:"192.168.1.0/24",rules:[{required:i&&i.address}]})}}),(0,e.jsx)(f.Z,{label:"\u914D\u7F6E ipV4 \u7F51\u5173",name:["ipV4","gateway"],width:"md",tooltip:"\u6307\u5B9A\u5BB9\u5668\u7684ipv4\u7684\u7F51\u5173\uFF0C\u4F8B\u5982 192.168.1.1",placeholder:"192.168.1.1"}),(0,e.jsx)(he.Z,{label:"\u81EA\u5B9A\u4E49ipV6",name:"enableIpV6Address"})]}),(0,e.jsx)(L.Z,{name:["enableIpV6Address"],children:function(I){var i=I.enableIpV6Address;if(i)return(0,e.jsxs)(X.UW,{children:[(0,e.jsx)(f.Z,{label:"\u914D\u7F6E ipV6 \u5730\u5740",name:["ipV6","address"],width:"md",tooltip:"\u6307\u5B9A\u5BB9\u5668\u7684ipV6\u5730\u5740\uFF0C\u4F8B\u5982 2001:db8::5",placeholder:"2001:db8::5"}),(0,e.jsx)(L.Z,{name:["ipV6"],children:function(l){var o=l.ipV6;return(0,e.jsx)(f.Z,{label:"\u914D\u7F6E ipV6 \u5B50\u7F51",name:["ipV6","subnet"],width:"md",required:o&&o.address,tooltip:"\u6307\u5B9A\u5BB9\u5668\u7684ipv4\u6240\u5728\u7684\u5B50\u7F51\uFF0C\u4F8B\u5982 2001:db8::/48",placeholder:"2001:db8::/48",rules:[{required:o&&o.address}]})}}),(0,e.jsx)(f.Z,{label:"\u914D\u7F6E ipV6 \u7F51\u5173",name:["ipV6","gateway"],width:"md",tooltip:"\u6307\u5B9A\u5BB9\u5668\u7684ipV6\u7684\u7F51\u5173\uFF0C\u4F8B\u5982 2001:db8::1",placeholder:"2001:db8::1"})]})}}),(0,e.jsx)(Te.Z,{label:"DNS\u914D\u7F6E",name:"dns",placeholder:"\u8BF7\u6307\u5B9Adns\u5730\u5740\uFF0C\u4F8B\u5982\uFF1A8.8.8.8\uFF0C\u4E00\u884C\u6DFB\u4E00\u6761dns\u5730\u5740"})]})}),du=su,cu=u(24963),ke=u(31199),mu=u(86250);function vu(){return(0,e.jsxs)(e.Fragment,{children:[(0,e.jsx)(x.Z,{title:"\u9644\u52A0\u6267\u884C\u811A\u672C",headerBordered:!0,subTitle:"",children:(0,e.jsx)(Te.Z,{label:"\u5BB9\u5668\u521B\u5EFA\u540E\u6267\u884C\u811A\u672C",name:["hook","containerCreate"],placeholder:"\u5BB9\u5668\u5728\u521B\u5EFA\u540E\u6267\u884C\u7684\u811A\u672C\uFF0C\u6B64\u811A\u672C\u5E76\u4E0D\u4F1A\u5F71\u54CD entrypoint \u53CA command\u3002\u4F8B\u5982\uFF1Als -al && touch abc.txt && apt update"})}),(0,e.jsxs)(x.Z,{title:"\u5065\u5EB7\u68C0\u67E5",headerBordered:!0,children:[(0,e.jsxs)(mu.Z,{gap:20,justify:"start",children:[(0,e.jsx)(ke.Z,{label:"\u91CD\u590D\u95F4\u9694\u65F6\u95F4\uFF08\u79D2\uFF09",name:["healthcheck","interval"],initialValue:30}),(0,e.jsx)(ke.Z,{label:"\u8D85\u65F6\u5931\u8D25\u65F6\u95F4\uFF08\u79D2\uFF09",name:["healthcheck","timeout"],initialValue:10}),(0,e.jsx)(ke.Z,{label:"\u5931\u8D25\u91CD\u590D\u6B21\u6570",name:["healthcheck","retries"],initialValue:3}),(0,e.jsx)(xe.Z,{width:"sm",label:"\u811A\u672C\u6267\u884C\u73AF\u5883",initialValue:"CMD",name:["healthcheck","shellType"],valueEnum:{CMD:"\u5728 Docker \u73AF\u5883\u6267\u884C","CMD-SHELL":"\u5728\u5BB9\u5668\u5185\u6267\u884C"}})]}),(0,e.jsx)(Te.Z,{label:"\u6267\u884C\u811A\u672C",name:["healthcheck","cmd"],placeholder:"\u914D\u7F6E\u5BB9\u5668\u5065\u5EB7\u68C0\u67E5\u811A\u672C\uFF0C\u4F8B\u5982\uFF1Acurl -f http://localhost:8080/ || exit 1"})]})]})}var pe="update",Ue="copy",we="new";function fu(){var r,y,B,I=(0,d.useContext)(cu.Z),i=I.loading,h=(0,d.useContext)(ue.Z),l=h.createFormRef,o=h.volumeListRef,C=h.domainRef,D=h.createEnvRef,p=h.createLinkRef,E=(0,d.useState)(we),V=U()(E,2),c=V[0],m=V[1],a=(0,Me.useSearchParams)(),j=U()(a,2),b=j[0],R=j[1],s=(0,Me.useNavigate)(),T=(0,d.useState)("basic"),N=U()(T,2),Z=N[0],ie=N[1],S=parseInt((r=b.get("id"))!==null&&r!==void 0?r:""),O=(y=b.get("containerId"))!==null&&y!==void 0?y:"",q=(B=b.get("imageId"))!==null&&B!==void 0?B:"";return(0,d.useEffect)(function(){if(O||S)i.show(),(0,Oe.iE)({md5:O,id:S}).then(function(){var A=W()(P()().mark(function n(t){var k,M,v,$,H,Ve,Ne,He;return P()().wrap(function(oe){for(;;)switch(oe.prev=oe.next){case 0:if(v={info:{},layer:[]},b.get("op")==Ue?m(Ue):m(pe),t.data.env.network&&t.data.env.network.map(function(F){return!F.alise&&F.name!="bridge"&&(F.alise=[t.data.siteName+".pod.dpanel.local"]),F}),t.data.env.ports&&t.data.env.ports.map(function(F){return F.host=="0"&&(F.host=""),F}),$=t.data.env.bindIpV6,t.data.containerInfo.Info&&t.data.containerInfo.Info.NetworkSettings.Networks&&Object.keys(t.data.containerInfo.Info.NetworkSettings.Networks).map(function(F){t.data.env.network&&(t.data.env.network=t.data.env.network.map(function(ee){return F==ee.name&&(ee.subnet=t.data.containerInfo.Info.NetworkSettings.Networks[F].IPAddress+"/"+t.data.containerInfo.Info.NetworkSettings.Networks[F].IPPrefixLen),ee})),t.data.containerInfo.Info.NetworkSettings.Networks[F].IPv6Gateway!=""&&($=!0)}),t.data.env.extraHosts&&t.data.env.extraHosts.map(function(F){if(F.value=="host-gateway"){var ee;(ee=l.current)===null||ee===void 0||ee.setFieldValue("enableBindHost",!0)}}),H=t.data.env.ports,t.data.env.ports&&(H=t.data.env.ports.map(function(F){return F.host=(F.hostIp?F.hostIp+":":"")+F.host,F})),(k=l.current)===null||k===void 0||k.setFieldsValue({imageName:t.data.env.imageName,privileged:t.data.env.privileged,bindIpV6:t.data.env.useHostNetwork?!1:$,useHostNetwork:t.data.env.useHostNetwork,publishAllPorts:t.data.env.useHostNetwork?!1:t.data.env.publishAllPorts,workDir:{value:t.data.env.workDir,useDefault:!t.data.env.workDir,default:v&&v.info.Config&&v.info.Config.WorkingDir},user:{value:t.data.env.user,useDefault:!t.data.env.user,default:v&&v.info.Config&&v.info.Config.User},command:{value:t.data.env.command,useDefault:!t.data.env.command,default:v&&v.info.Config&&v.info.Config.Cmd&&v.info.Config.Cmd.join(" ")},entrypoint:{value:t.data.env.entrypoint,useDefault:!t.data.env.entrypoint,default:v&&v.info.Config&&v.info.Config.Entrypoint&&v.info.Config.Entrypoint.join(" ")},shmsize:(M=t.data.env.shmsize)!==null&&M!==void 0?M:"64M",cpus:t.data.env.cpus,memory:t.data.env.memory,environment:t.data.env.environment,label:t.data.env.label,volumesDefault:t.data.env.volumesDefault,volumes:t.data.env.volumes,ports:H,links:t.data.env.links,network:t.data.env.network,restart:t.data.env.restart,extraHosts:t.data.env.extraHosts,autoRemove:t.data.env.autoRemove,log:t.data.env.log,dns:t.data.env.dns&&t.data.env.dns.join(`
`),ipV4:t.data.env.ipV4,ipV6:t.data.env.ipV6,gpus:t.data.env.gpus,device:t.data.env.device,hook:t.data.env.hook}),t.data.env.healthcheck&&t.data.env.healthcheck.cmd&&t.data.env.healthcheck.cmd!=""&&((Ve=l.current)===null||Ve===void 0||Ve.setFieldsValue({healthcheck:t.data.env.healthcheck})),b.get("op")==pe&&((Ne=l.current)===null||Ne===void 0||Ne.setFieldsValue({siteTitle:t.data.siteTitle,siteName:t.data.siteName})),!t.data.env.imageName){oe.next=17;break}return oe.next=15,(0,J.YU)({md5:t.data.env.imageName});case 15:He=oe.sent,v=He.data;case 17:case"end":return oe.stop()}},n)}));return function(n){return A.apply(this,arguments)}}()).finally(function(){i.destroy()});else{var w;m(we),(w=l.current)===null||w===void 0||w.resetFields()}},[b]),(0,e.jsx)(ye._z,{children:(0,e.jsx)(x.Z,{direction:"column",gutter:[0,10],children:(0,e.jsxs)(Ie.A,{submitter:{render:function(A,n){return(0,e.jsx)(K.S,{children:n})}},formRef:l,onFinish:function(){var w=W()(P()().mark(function A(n){var t,k,M,v;return P()().wrap(function(H){for(;;)switch(H.prev=H.next){case 0:return console.log(n),v={siteTitle:n.siteTitle,siteName:n.siteName,imageName:n.imageName,environment:n.environment,links:n.links,ports:n.ports,volumes:n.volumes,volumesDefault:n.volumesDefault,network:n.network,privileged:(t=n.privileged)!==null&&t!==void 0?t:!1,autoRemove:(k=n.autoRemove)!==null&&k!==void 0?k:!1,restart:n.restart,cpus:n.cpus,memory:n.memory,shmsize:(M=n.shmsize)!==null&&M!==void 0?M:0,workDir:n.workDir&&!n.workDir.useDefault?n.workDir.value:"",user:n.user&&!n.user.useDefault?n.user.value:"",command:n.command&&!n.command.useDefault?n.command.value:"",entrypoint:n.entrypoint&&!n.entrypoint.useDefault?n.entrypoint.value:"",useHostNetwork:n.useHostNetwork,bindIpV6:n.bindIpV6,log:n.log,dns:n.dns&&n.dns!=""?n.dns.split(`
`):[],label:n.label,publishAllPorts:n.publishAllPorts,extraHosts:n.extraHosts,ipV4:n.ipV4,ipV6:n.ipV6,device:n.device,gpus:n.gpus,hook:n.hook},n.healthcheck&&n.healthcheck.cmd&&n.healthcheck.cmd!=""&&(v.healthcheck=n.healthcheck),O&&c==pe&&(v.containerId=O),H.next=6,(0,Oe.$G)(v);case 6:return s("/app/list"),H.abrupt("return",!0);case 8:case"end":return H.stop()}},A)}));return function(A){return w.apply(this,arguments)}}(),children:[(0,e.jsxs)(x.Z,{title:"\u57FA\u7840\u4FE1\u606F",headerBordered:!0,children:[(0,e.jsx)(f.Z,{name:"siteTitle",label:"\u7AD9\u70B9\u540D\u79F0",required:!0,rules:[{required:!0}],fieldProps:{onChange:function(A){var n="";if(A.target.value&&c!=pe){var t,k=(0,ru.N9)(A.target.value.trim(),{toneType:"none",type:"array"});n=k.join(""),(t=l.current)===null||t===void 0||t.setFieldValue("siteName",n)}}},placeholder:"\u8BF7\u8F93\u5165\u7AD9\u70B9\u540D\u79F0"}),(0,e.jsx)(f.Z,{name:"siteName",label:"\u7AD9\u70B9\u6807\u8BC6",tooltip:"\u7AD9\u70B9\u552F\u4E00\u6807\u8BC6\uFF0C\u7528\u4E8E\u6807\u8BC6\u7AD9\u70B9\u548C\u5185\u90E8\u8BBF\u95EE",required:!0,disabled:c==pe,rules:[{required:!0}],placeholder:"\u8BF7\u8F93\u5165\u7AD9\u70B9\u540D\u79F0"}),(0,e.jsx)(Se,{redeploy:c!=we,fromImageId:q})]}),(0,e.jsx)(be.Z,{offsetTop:50,children:(0,e.jsx)(x.Z,{style:{marginBottom:-20},children:(0,e.jsx)(Ee.Z,{activeKey:Z,onChange:function(A){ie(A),window.scrollTo(0,450)},items:[{label:"\u57FA\u672C\u914D\u7F6E",key:"basic"},{label:"\u5173\u8054\u914D\u7F6E",key:"link"},{label:"\u5B58\u50A8\u914D\u7F6E",key:"storage"},{label:"\u8FD0\u884C\u914D\u7F6E",key:"run-command"},{label:"\u8D44\u6E90\u914D\u7F6E",key:"resource"},{label:"Hook",key:"hook"},{label:"\u5176\u5B83",key:"other"}]})})}),(0,e.jsxs)(_,{show:Z=="basic",children:[(0,e.jsx)(iu.Z,{ref:C,showBindHost:!0,showBindIpV6:!0}),(0,e.jsx)(Je.Z,{showAddButton:!0,showImportButton:!0,ref:D}),(0,e.jsx)(L.Z,{name:["siteName","useHostNetwork"],children:function(A){var n=A.siteName,t=A.useHostNetwork;if(!t)return(0,e.jsx)(du,{})}})]}),(0,e.jsx)(_,{show:Z=="link",children:(0,e.jsx)(L.Z,{name:["siteName","useHostNetwork"],children:function(A){var n=A.siteName,t=A.useHostNetwork;return t?(0,e.jsx)(G.Z,{showIcon:!0,description:"\u7ED1\u5B9A\u5230\u5BBF\u4E3B\u673A\u7F51\u7EDC\u65F6\uFF0C\u65E0\u6CD5\u901A\u8FC7 Docker \u5173\u8054\u5176\u5B83\u5BB9\u5668\u3002\u8BF7\u4F7F\u7528\u5BBF\u4E3B\u673A\u5185\u7F51IP\u6216\u662F127.0.0.1\u4E92\u8054\u5BB9\u5668\u66B4\u9732\u7AEF\u53E3\u3002"}):(0,e.jsxs)(e.Fragment,{children:[(0,e.jsx)(ze,{onAddEnv:function(M,v){var $;($=D.current)===null||$===void 0||$.addEnvItem(M,v)},ref:p}),(0,e.jsx)(lu.Z,{siteName:n}),(0,e.jsx)(ou,{})]})}})}),(0,e.jsx)(_,{show:Z=="storage",children:(0,e.jsx)(Qe.Z,{showDefault:!0,ref:o})}),(0,e.jsx)(_,{show:Z=="run-command",children:(0,e.jsx)(au,{})}),(0,e.jsx)(_,{show:Z=="resource",children:(0,e.jsx)(qe,{})}),(0,e.jsx)(_,{show:Z=="hook",children:(0,e.jsx)(vu,{})}),(0,e.jsx)(_,{show:Z=="other",children:(0,e.jsx)(eu,{})})]},"form")})})}}}]);